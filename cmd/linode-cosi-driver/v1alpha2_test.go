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
	"io"
	"log/slog"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	cosi "sigs.k8s.io/container-object-storage-interface/proto"

	"github.com/linode/linode-cosi-driver/pkg/servers/identity"
	"github.com/linode/linode-cosi-driver/pkg/servers/provisioner"
)

func TestV1alpha2GRPCServices(t *testing.T) {
	t.Parallel()
	identityServer, err := identity.New("objectstorage.cosi.linode.com")
	if err != nil {
		t.Fatal(err)
	}
	server, err := grpcServer(t.Context(), slog.New(slog.NewTextHandler(io.Discard, nil)), identityServer, &provisioner.Server{})
	if err != nil {
		t.Fatal(err)
	}
	listener := bufconn.Listen(1024 * 1024)
	t.Cleanup(server.Stop)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			t.Errorf("serve: %v", err)
		}
	}()
	connection, err := grpc.NewClient("passthrough:///cosi-test",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := connection.Close(); err != nil {
			t.Errorf("close connection: %v", err)
		}
	})
	info, err := cosi.NewIdentityClient(connection).DriverGetInfo(t.Context(), &cosi.DriverGetInfoRequest{})
	if err != nil || info.GetName() != "objectstorage.cosi.linode.com" || len(info.GetSupportedProtocols()) != 1 {
		t.Fatalf("v1alpha2 identity discovery failed: %v", err)
	}
	generated, err := cosi.NewProvisionerClient(connection).DriverGenerateBucketId(t.Context(), &cosi.DriverGenerateBucketIdRequest{
		Name: "example-bucket", Parameters: map[string]string{provisioner.ParamRegion: "us-ord"},
	})
	if err != nil || generated.GetBucketId() != "us-ord/example-bucket" {
		t.Fatalf("v1alpha2 bucket ID generation failed: %v", err)
	}
	err = connection.Invoke(t.Context(), "/cosi.v1alpha1.Identity/DriverGetInfo", &cosi.DriverGetInfoRequest{}, &cosi.DriverGetInfoResponse{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("v1alpha1 service should not be registered: %v", err)
	}
}
