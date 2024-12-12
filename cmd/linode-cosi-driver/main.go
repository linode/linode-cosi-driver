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
	"sync"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"go.uber.org/automaxprocs/maxprocs"
	"google.golang.org/grpc"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"

	"github.com/linode/linode-cosi-driver/pkg/endpoint"
	"github.com/linode/linode-cosi-driver/pkg/envflag"
	grpchandlers "github.com/linode/linode-cosi-driver/pkg/grpc"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/pkg/logutils"
	"github.com/linode/linode-cosi-driver/pkg/servers/identity"
	"github.com/linode/linode-cosi-driver/pkg/servers/provisioner"
	"github.com/linode/linode-cosi-driver/pkg/version"
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

	if err := run(context.Background(), log, mainOptions{
		cosiEndpoint:     cosiEndpoint,
		linodeToken:      linodeToken,
		linodeURL:        linodeURL,
		linodeAPIVersion: linodeAPIVersion,
	},
	); err != nil {
		slog.Error("Critical failure", "error", err)
		os.Exit(1)
	}
}

type mainOptions struct {
	cosiEndpoint     string
	linodeToken      string
	linodeURL        string
	linodeAPIVersion string
}

func run(ctx context.Context, log *slog.Logger, opts mainOptions) error {
	_, err := maxprocs.Set(maxprocs.Logger(logutils.ForMaxprocs(log.Handler())))
	if err != nil {
		return fmt.Errorf("setting GOMAXPROCS failed: %w", err)
	}

	ctx, stop := signal.NotifyContext(ctx,
		os.Interrupt,
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	// create identity server
	idSrv, err := identity.New(driverName)
	if err != nil {
		return fmt.Errorf("failed to create identity server: %w", err)
	}

	// initialize Linode client
	client, err := linodeclient.NewLinodeClient(
		opts.linodeToken,
		fmt.Sprintf("LinodeCOSI/%s", version.Version),
		opts.linodeURL,
		opts.linodeAPIVersion)
	if err != nil {
		return fmt.Errorf("unable to create new client: %w", err)
	}

	client.SetLogger(logutils.ForResty(log))

	// create provisioner server
	prvSrv, err := provisioner.New(
		log,
		client,
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

	slog.Info("Starting server",
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
		grpc.ChainUnaryInterceptor(
			logging.UnaryServerInterceptor(logutils.ForGRPC(log.Handler())),
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpchandlers.PanicRecovery(ctx, log.Handler()))),
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
	defer slog.Info("Stopped")

	slog.Info("Shutting down")

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
				slog.Warn("Forcing shutdown")
				g.Stop()

				return
			case <-c:
				return
			}
		}
	}
}
