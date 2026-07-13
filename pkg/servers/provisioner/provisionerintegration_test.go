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

	"github.com/linode/linodego/v2"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"

	"github.com/linode/linode-cosi-driver/pkg/envflag"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient/cache"
	"github.com/linode/linode-cosi-driver/pkg/servers/provisioner"
	"github.com/linode/linode-cosi-driver/pkg/version"
)

const integrationBucketPolicyTemplate = `{
	"Version":"2012-10-17",
	"Statement":[
		{
			"Effect":"Allow",
			"Action":"*",
			"Resource":[
			"arn:aws:s3:::{{ .BucketName }}",
			"arn:aws:s3:::{{ .BucketName }}/*"
			],
			"Principal":"*"
		}
	]
}`

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

	region, err := resolveTestRegion(t.Context(), client, "", true)
	if err != nil {
		t.Errorf("failed to resolve test region: %v", err)
		return
	}

	testCache := cache.New(slog.Default(), client, cache.DefaultTTL)
	if err := testCache.Refresh(t.Context()); err != nil {
		t.Errorf("failed to refresh cache: %v", err.Error())
		return
	}

	srv, err := provisioner.New(slog.Default(), client, testCache, nil, true)
	if err != nil {
		t.Errorf("failed to create provisioner: %v", err.Error())
		return
	}

	suffix := time.Now().UnixNano()
	suite := suite{
		server:     srv,
		region:     region,
		bucketName: fmt.Sprintf("integration-%d", suffix),
		accessName: fmt.Sprintf("integration-access-%d", suffix),
	}

	idempotentRun(t, iterations, "DriverCreateBucket", suite.DriverCreateBucket)
	idempotentRun(t, iterations, "DriverGrantBucketAccess", suite.DriverGrantBucketAccess)
	idempotentRun(t, iterations, "DriverRevokeBucketAccess", suite.DriverRevokeBucketAccess)
	idempotentRun(t, iterations, "DriverDeleteBucket", suite.DriverDeleteBucket)
}

func TestBucketScopedKeyIsolation(t *testing.T) {
	t.Parallel()

	var (
		linodeToken = envflag.String("LINODE_TOKEN", "")
		region      = envflag.String("OBJ_TEST_REGION", "")
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

	region, err = resolveTestRegion(t.Context(), client, region, false)
	if err != nil {
		t.Errorf("failed to resolve test region: %v", err)
		return
	}

	testCache := cache.New(slog.Default(), client, cache.DefaultTTL)
	if err := testCache.Refresh(t.Context()); err != nil {
		t.Errorf("failed to refresh cache: %v", err.Error())
		return
	}

	srv, err := provisioner.New(slog.Default(), client, testCache, nil, true)
	if err != nil {
		t.Errorf("failed to create provisioner: %v", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	bucketA := fmt.Sprintf("integration-a-%d", time.Now().UnixNano())
	bucketB := fmt.Sprintf("integration-b-%d", time.Now().UnixNano())

	createBucket := func(name string) {
		req := &cosi.DriverCreateBucketRequest{
			Name: name,
			Parameters: map[string]string{
				provisioner.ParamRegion: region,
				provisioner.ParamACL:    "private",
			},
		}
		if _, err := srv.DriverCreateBucket(ctx, req); err != nil {
			t.Fatalf("failed to create bucket %s: %v", name, err)
		}
	}

	deleteBucket := func(name string) {
		req := &cosi.DriverDeleteBucketRequest{
			BucketId: fmt.Sprintf("%s/%s", region, name),
		}
		if _, err := srv.DriverDeleteBucket(ctx, req); err != nil {
			t.Errorf("failed to delete bucket %s: %v", name, err)
		}
	}

	createBucket(bucketA)
	createBucket(bucketB)
	defer deleteBucket(bucketA)
	defer deleteBucket(bucketB)

	grantReq := &cosi.DriverGrantBucketAccessRequest{
		BucketId:           fmt.Sprintf("%s/%s", region, bucketA),
		Name:               "integration-access",
		AuthenticationType: cosi.AuthenticationType_Key,
		Parameters: map[string]string{
			provisioner.ParamPermissions: string(provisioner.ParamPermissionsValueReadWrite),
		},
	}

	grantRes, err := srv.DriverGrantBucketAccess(ctx, grantReq)
	if err != nil {
		t.Fatalf("failed to grant bucket access: %v", err)
	}
	defer func() {
		revokeReq := &cosi.DriverRevokeBucketAccessRequest{
			BucketId:  fmt.Sprintf("%s/%s", region, bucketA),
			AccountId: grantRes.GetAccountId(),
		}
		if _, err := srv.DriverRevokeBucketAccess(ctx, revokeReq); err != nil {
			t.Errorf("failed to revoke bucket access: %v", err)
		}
	}()

	creds := grantRes.GetCredentials()[provisioner.S3]
	if creds == nil {
		t.Fatalf("missing s3 credentials in response")
	}
	endpoint := creds.Secrets[provisioner.S3Endpoint]
	if endpoint == "" {
		t.Fatalf("missing endpoint in response")
	}
	accessKey := creds.Secrets[provisioner.S3SecretAccessKeyID]
	secretKey := creds.Secrets[provisioner.S3SecretAccessSecretKey]
	if accessKey == "" || secretKey == "" {
		t.Fatalf("missing access credentials in response")
	}

	s3cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Region: region,
		Secure: true,
	})
	if err != nil {
		t.Fatalf("failed to create s3 client: %v", err)
	}

	if err := waitForListObjects(ctx, s3cli, bucketA); err != nil {
		t.Fatalf("expected access to bucketA, got error: %v", err)
	}

	err = listObjectsErr(ctx, s3cli, bucketB)
	if err == nil {
		t.Fatalf("expected access denied for bucketB, got nil")
	}
	if resp := minio.ToErrorResponse(err); resp.StatusCode != 0 && resp.StatusCode != 403 {
		t.Fatalf("expected access denied for bucketB, got: %v", err)
	}
}

func listObjectsErr(ctx context.Context, cli *minio.Client, bucket string) error {
	for obj := range cli.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true}) {
		if obj.Err != nil {
			return obj.Err
		}
	}
	return nil
}

func waitForListObjects(ctx context.Context, cli *minio.Client, bucket string) error {
	waitCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var lastErr error
	for {
		if err := listObjectsErr(waitCtx, cli, bucket); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case <-waitCtx.Done():
			return fmt.Errorf("timed out waiting for bucket access: %w", lastErr)
		case <-ticker.C:
		}
	}
}

func resolveTestRegion(ctx context.Context, client linodeclient.Client, requested string, requireCORS bool) (string, error) {
	if requested != "" && !requireCORS {
		return requested, nil
	}

	endpoints, err := client.ListObjectStorageEndpoints(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("list object storage endpoints: %w", err)
	}

	if requested != "" {
		if hasCORSEndpoint(endpoints, requested) {
			return requested, nil
		}

		return "", fmt.Errorf("requested region %s does not have an object storage endpoint that supports CORS", requested)
	}

	for _, endpoint := range endpoints {
		if endpoint.Region == "" || endpoint.S3Endpoint == nil || *endpoint.S3Endpoint == "" {
			continue
		}
		if requireCORS && !endpointTypeSupportsCORS(endpoint.EndpointType) {
			continue
		}

		return endpoint.Region, nil
	}

	if requireCORS {
		return "", fmt.Errorf("no object storage endpoint with CORS support available")
	}

	return "", fmt.Errorf("no object storage endpoint with region and S3 endpoint available")
}

func hasCORSEndpoint(endpoints []linodego.ObjectStorageEndpoint, region string) bool {
	for _, endpoint := range endpoints {
		if endpoint.Region == region &&
			endpoint.S3Endpoint != nil &&
			*endpoint.S3Endpoint != "" &&
			endpointTypeSupportsCORS(endpoint.EndpointType) {
			return true
		}
	}

	return false
}

func endpointTypeSupportsCORS(endpointType linodego.ObjectStorageEndpointType) bool {
	return endpointType != linodego.ObjectStorageEndpointE2 &&
		endpointType != linodego.ObjectStorageEndpointE3
}

type suite struct {
	server *provisioner.Server

	finishedCreateBucket      bool
	finishedGrantBucketAccess bool

	region     string
	bucketName string
	accessName string
	bucketID   string
	accountID  string
}

func (s *suite) DriverCreateBucket(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	req := &cosi.DriverCreateBucketRequest{
		Name: s.bucketName,
		Parameters: map[string]string{
			provisioner.ParamRegion: s.region,
			provisioner.ParamACL:    "private",
			provisioner.ParamCORS:   string(provisioner.ParamCORSValueEnabled),
			provisioner.ParamPolicy: integrationBucketPolicyTemplate,
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
		Name:               s.accessName,
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
