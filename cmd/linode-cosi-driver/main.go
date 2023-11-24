// Copyright 2023 Akamai Technologies, Inc.
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
	"github.com/linode/linode-cosi-driver/pkg/endpoint"
	"github.com/linode/linode-cosi-driver/pkg/envflag"
	"github.com/linode/linode-cosi-driver/pkg/grpc/handlers"
	"github.com/linode/linode-cosi-driver/pkg/grpc/logger"
	"github.com/linode/linode-cosi-driver/pkg/servers/identity"
	"github.com/linode/linode-cosi-driver/pkg/servers/provisioner"
	"google.golang.org/grpc"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"
)

var log *slog.Logger

const (
	driverName  = "objectstorage.cosi.linode.com"
	gracePeriod = 5 * time.Second
)

func main() {
	linodeToken := envflag.String("LINODE_TOKEN", "")
	linodeURL := envflag.String("LINODE_API_URL", "")
	cosiEndpoint := envflag.String("COSI_ENDPOINT", "unix:///var/lib/cosi/cosi.sock")

	// TODO: any logger settup must be done here, before first log call.
	log = slog.Default()

	if err := realMain(context.Background(), cosiEndpoint, linodeToken, linodeURL); err != nil {
		slog.Error("critical failure", "error", err)
		os.Exit(1)
	}
}

func realMain(ctx context.Context, cosiEndpoint, _, _ string) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// create identity server
	idSrv, err := identity.New(driverName)
	if err != nil {
		return fmt.Errorf("failed to create identity server: %w", err)
	}

	// create provisioner server
	prvSrv, err := provisioner.New(log)
	if err != nil {
		return fmt.Errorf("failed to create provisioner server: %w", err)
	}

	// parse endpoint
	endpointURL, err := url.Parse(cosiEndpoint)
	if err != nil {
		return fmt.Errorf("unable to parse COSI endpoint: %w", err)
	}

	// create the endpoint handler
	lis, err := endpoint.New(endpointURL).Listener(ctx)
	if err != nil {
		return fmt.Errorf("unable to create new listener: %w", err)
	}
	defer lis.Close()

	// create the grpcServer
	srv, err := grpcServer(ctx, idSrv, prvSrv)
	if err != nil {
		return fmt.Errorf("gRPC server creation failed: %w", err)
	}

	var wg sync.WaitGroup

	wg.Add(1)

	go shutdown(ctx, &wg, srv)

	slog.Info("starting server", "endpoint", endpointURL)

	err = srv.Serve(lis)
	if err != nil {
		return fmt.Errorf("gRPC server failed: %w", err)
	}

	wg.Wait()

	return nil
}

func grpcServer(ctx context.Context, identity cosi.IdentityServer, provisioner cosi.ProvisionerServer) (*grpc.Server, error) {
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			logging.UnaryServerInterceptor(logger.Wrap(log)),
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(handlers.PanicRecovery(ctx, log))),
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
func shutdown(ctx context.Context, wg *sync.WaitGroup, g *grpc.Server) {
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
