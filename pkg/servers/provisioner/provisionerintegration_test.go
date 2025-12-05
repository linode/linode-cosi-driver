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

//go:build integration

package provisioner_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	cosi "sigs.k8s.io/container-object-storage-interface-spec"

	"github.com/linode/linode-cosi-driver/pkg/envflag"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient/cache"
	"github.com/linode/linode-cosi-driver/pkg/s3"
	"github.com/linode/linode-cosi-driver/pkg/servers/provisioner"
	"github.com/linode/linode-cosi-driver/pkg/version"
)

func idempotentRun(t *testing.T, n int, name string, run func(t *testing.T)) {
	for i := 0; i < n; i++ {
		t.Run(fmt.Sprintf("%s_%d", name, i), run)
	}
}

func TestHappyPath(t *testing.T) {
	t.Parallel()

	var (
		linodeToken = envflag.String("LINODE_TOKEN", "")
		iterations  = envflag.Int("IDEMPOTENCY_ITERATIONS", 2)
	)

	if linodeToken == "" {
		t.Errorf("LINODE_TOKEN not set")
		return
	}

	client, err := linodeclient.NewLinodeClient(fmt.Sprintf("LinodeCOSI/%s+integration", version.Version))
	if err != nil {
		t.Errorf("failed to create client: %v", err.Error())
		return
	}

	testCache := cache.New(slog.Default(), client, cache.DefaultTTL)
	if err := testCache.Refresh(t.Context()); err != nil {
		t.Errorf("failed to refresh cache: %v", err.Error())
		return
	}

	creds, cleanup, err := linodeclient.NewEphemeralS3Credentials(context.Background(), slog.Default(), client)
	if err != nil {
		t.Errorf("failed to create ephemeral s3 credentials: %v", err.Error())
		return
	}

	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		if err := cleanup(ctx); err != nil {
			t.Errorf("unable to cleanup ephemeral credentials: %v", err)
		}
	}()

	s3cli := s3.New(
		testCache,
		creds.AccessKey, creds.SecretKey,
		true,
	)

	srv, err := provisioner.New(slog.Default(), client, testCache, s3cli)
	if err != nil {
		t.Errorf("failed to create provisioner: %v", err.Error())
		return
	}

	suite := suite{
		server: srv,
	}

	idempotentRun(t, iterations, "DriverCreateBucket", suite.DriverCreateBucket)
	idempotentRun(t, iterations, "DriverGrantBucketAccess", suite.DriverGrantBucketAccess)
	idempotentRun(t, iterations, "DriverRevokeBucketAccess", suite.DriverRevokeBucketAccess)
	idempotentRun(t, iterations, "DriverDeleteBucket", suite.DriverDeleteBucket)
}

type suite struct {
	server *provisioner.Server

	finishedCreateBucket      bool
	finishedGrantBucketAccess bool

	bucketID  string
	accountID string
}

func (s *suite) DriverCreateBucket(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	req := &cosi.DriverCreateBucketRequest{
		Name: "integration",
		Parameters: map[string]string{
			provisioner.ParamRegion: "us-east",
			provisioner.ParamACL:    "private",
			provisioner.ParamCORS:   string(provisioner.ParamCORSValueEnabled),
		},
	}

	res, err := s.server.DriverCreateBucket(ctx, req)
	if err != nil {
		t.Errorf("failed to create bucket: %v", err)
	} else {
		s.bucketID = res.GetBucketId()
		s.finishedCreateBucket = true
	}
}

func (s *suite) DriverDeleteBucket(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	if !s.finishedCreateBucket {
		t.Errorf("DriverCreateBucket not executed successfully")
		return
	}

	req := &cosi.DriverDeleteBucketRequest{
		BucketId: s.bucketID,
	}

	_, err := s.server.DriverDeleteBucket(ctx, req)
	if err != nil {
		t.Errorf("failed to delete bucket: %v", err)
	}
}

func (s *suite) DriverGrantBucketAccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	if !s.finishedCreateBucket {
		t.Errorf("DriverCreateBucket not executed successfully")
		return
	}

	req := &cosi.DriverGrantBucketAccessRequest{
		BucketId:           s.bucketID,
		Name:               "integration",
		AuthenticationType: cosi.AuthenticationType_Key,
		Parameters: map[string]string{
			provisioner.ParamPermissions: string(provisioner.ParamPermissionsValueReadWrite),
		},
	}

	res, err := s.server.DriverGrantBucketAccess(ctx, req)
	if err != nil {
		t.Errorf("failed to grant bucket access: %v", err)
	} else {
		s.accountID = res.GetAccountId()
		s.finishedGrantBucketAccess = true
	}
}

func (s *suite) DriverRevokeBucketAccess(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	if !s.finishedCreateBucket || !s.finishedGrantBucketAccess {
		t.Errorf("DriverCreateBucket or DriverGrantBucketAccess not executed successfully")
		return
	}

	req := &cosi.DriverRevokeBucketAccessRequest{
		BucketId:  s.bucketID,
		AccountId: s.accountID,
	}

	_, err := s.server.DriverRevokeBucketAccess(ctx, req)
	if err != nil {
		t.Errorf("failed to revoke bucket access: %v", err)
	}
}
