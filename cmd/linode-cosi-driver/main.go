// Copyright 2023-2024 Akamai Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/linode/linode-cosi-driver/pkg/endpoint"
	"github.com/linode/linode-cosi-driver/pkg/envflag"
	"github.com/linode/linode-cosi-driver/pkg/grpc/handlers"
	grpclogger "github.com/linode/linode-cosi-driver/pkg/grpc/logger"
	"github.com/linode/linode-cosi-driver/pkg/kubereader/tracedkubereader"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient/tracedclient"
	maxprocslogger "github.com/linode/linode-cosi-driver/pkg/maxprocs/logger"
	"github.com/linode/linode-cosi-driver/pkg/observability/metrics"
	"github.com/linode/linode-cosi-driver/pkg/observability/tracing"
	restylogger "github.com/linode/linode-cosi-driver/pkg/resty/logger"
	"github.com/linode/linode-cosi-driver/pkg/servers/identity"
	"github.com/linode/linode-cosi-driver/pkg/servers/provisioner"
	"github.com/linode/linode-cosi-driver/pkg/version"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.uber.org/automaxprocs/maxprocs"
	"google.golang.org/grpc"
	v1 "k8s.io/api/core/v1"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	ctrlLog "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	driverName     = "objectstorage.cosi.linode.com"
	gracePeriod    = 5 * time.Second
	envK8sNodeName = "K8S_NODE_NAME"
	envK8sPodName  = "K8S_POD_NAME"
)

func main() {
	var (
		linodeToken      = envflag.String("LINODE_TOKEN", "")
		linodeURL        = envflag.String("LINODE_API_URL", "")
		linodeAPIVersion = envflag.String("LINODE_API_VERSION", "")
		cosiEndpoint     = envflag.String("COSI_ENDPOINT", "unix:///var/lib/cosi/cosi.sock")
	)

	// TODO: any logger settup must be done here, before first log call.
	log := slog.Default()
	ctrlLog.SetLogger(logr.FromSlogHandler(log.Handler()))

	kubeclient, err := client.New(config.GetConfigOrDie(), client.Options{
		Cache: &client.CacheOptions{
			DisableFor: []client.Object{
				&v1.Secret{},
			},
		},
	})
	if err != nil {
		slog.Error("initializing kube client failed", "error", err)
		os.Exit(1)
	}

	if err := run(context.Background(), log, mainOptions{
		cosiEndpoint:     cosiEndpoint,
		linodeToken:      linodeToken,
		linodeURL:        linodeURL,
		linodeAPIVersion: linodeAPIVersion,
		kubeclient:       kubeclient,
	},
	); err != nil {
		slog.Error("critical failure", "error", err)
		os.Exit(1)
	}
}

type mainOptions struct {
	cosiEndpoint     string
	linodeToken      string
	linodeURL        string
	linodeAPIVersion string
	kubeclient       client.Client
}

func run(ctx context.Context, log *slog.Logger, opts mainOptions) error {
	_, err := maxprocs.Set(maxprocs.Logger(maxprocslogger.Wrap(log.Handler())))
	if err != nil {
		return fmt.Errorf("setting GOMAXPROCS failed: %w", err)
	}

	ctx, stop := signal.NotifyContext(ctx,
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	o11yShutdown := setupObservabillity(ctx, log)
	defer o11yShutdown()

	// create identity server
	idSrv, err := identity.New(driverName)
	if err != nil {
		return fmt.Errorf("failed to create identity server: %w", err)
	}

	// initialize Linode client
	client, err := linodeclient.NewLinodeClient(
		opts.linodeToken,
		version.UserAgent(),
		opts.linodeURL,
		opts.linodeAPIVersion)
	if err != nil {
		return fmt.Errorf("unable to create new client: %w", err)
	}

	client.SetLogger(restylogger.Wrap(log))

	podName := os.Getenv(envK8sPodName)

	// create provisioner server
	prvSrv, err := provisioner.New(
		log,
		tracedclient.NewClientWithTracing(client, podName),
		tracedkubereader.NewKubeReaderWithTracing(opts.kubeclient, podName),
	)
	if err != nil {
		return fmt.Errorf("failed to create provisioner server: %w", err)
	}

	// parse endpoint
	endpointURL, err := url.Parse(opts.cosiEndpoint)
	if err != nil {
		return fmt.Errorf("unable to parse COSI endpoint: %w", err)
	}

	// create the endpoint handler
	ep := endpoint.New(endpointURL)
	defer ep.Close()

	lis, err := ep.Listener(ctx)
	if err != nil {
		return fmt.Errorf("unable to create new listener: %w", err)
	}
	defer lis.Close()

	// create the grpcServer
	srv, err := grpcServer(ctx, log, idSrv, prvSrv)
	if err != nil {
		return fmt.Errorf("gRPC server creation failed: %w", err)
	}

	var wg sync.WaitGroup

	wg.Add(1)

	go shutdown(ctx, &wg, srv)

	slog.Info("starting server",
		"endpoint", endpointURL,
		"version", version.Version)

	err = srv.Serve(lis)
	if err != nil {
		return fmt.Errorf("gRPC server failed: %w", err)
	}

	wg.Wait()

	return nil
}

func grpcServer(ctx context.Context,
	log *slog.Logger,
	identity cosi.IdentityServer,
	provisioner cosi.ProvisionerServer,
) (*grpc.Server, error) {
	server := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			logging.UnaryServerInterceptor(grpclogger.Wrap(log.Handler())),
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(handlers.PanicRecovery(ctx, log.Handler()))),
		),
	)

	if identity == nil || provisioner == nil {
		return nil, errors.New("provisioner and identity servers cannot be nil")
	}

	cosi.RegisterIdentityServer(server, identity)
	cosi.RegisterProvisionerServer(server, provisioner)

	return server, nil
}

// shutdown handles shutdown with grace period consideration.
func shutdown(ctx context.Context,
	wg *sync.WaitGroup,
	g *grpc.Server,
) {
	<-ctx.Done()
	defer wg.Done()
	defer slog.Info("stopped")

	slog.Info("shutting down")

	dctx, stop := context.WithTimeout(context.Background(), gracePeriod)
	defer stop()

	c := make(chan struct{})

	if g != nil {
		go func() {
			g.GracefulStop()
			c <- struct{}{}
		}()

		for {
			select {
			case <-dctx.Done():
				slog.Warn("forcing shutdown")
				g.Stop()

				return
			case <-c:
				return
			}
		}
	}
}

func setupObservabillity(ctx context.Context, log *slog.Logger) func() {
	node := os.Getenv(envK8sNodeName)
	pod := os.Getenv(envK8sPodName)

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(driverName),
		semconv.ServiceVersion(version.Version),
		semconv.K8SPodName(pod),
		semconv.K8SNodeName(node),
	)

	tracingShutdown, err := tracing.Setup(ctx, res)
	if err != nil {
		// non critical error, just log it.
		log.Error("failed to setup tracing",
			"error", err,
		)
	}

	metricsShutdown, err := metrics.Setup(ctx, res)
	if err != nil {
		// non critical error, just log it.
		log.Error("failed to setup metrics",
			"error", err,
		)
	}

	attrs := []any{}

	for _, kv := range os.Environ() {
		k, v, ok := strings.Cut(kv, "=")
		if ok && strings.HasPrefix(k, "OTEL_") {
			attrs = append(attrs, slog.String(k, v))
		}
	}

	log.Info("opentelemetry configuration applied",
		attrs...,
	)

	return func() {
		timeout := 25 * time.Second // nolint:mnd // 2.5x default OTLP timeout

		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		defer cancel()

		wg := &sync.WaitGroup{}

		if tracingShutdown != nil {
			wg.Add(1)

			go func() {
				defer wg.Done()

				if err := tracingShutdown(ctx); err != nil {
					log.Error("failed to shutdown tracing",
						"error", err,
					)
				}
			}()
		}

		if metricsShutdown != nil {
			wg.Add(1)

			go func() {
				defer wg.Done()

				if err := tracingShutdown(ctx); err != nil {
					log.Error("failed to shutdown tracing",
						"error", err,
					)
				}
			}()
		}

		wg.Wait()
	}
}
