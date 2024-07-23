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

package provisioner_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient/stubclient"
	"github.com/linode/linode-cosi-driver/pkg/servers/provisioner"
	"github.com/linode/linode-cosi-driver/pkg/testutils"
	"github.com/linode/linodego"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"
)

const (
	testRegion           = "test-region"
	testBucketName       = "test-bucket"
	testBucketID         = testRegion + "/" + testBucketName
	testBucketAccessName = "test-bucket-access"
	testBucketAccessID   = "0"
)

var (
	defaultLinodegoBucket = &linodego.ObjectStorageBucket{
		Label:  testBucketName,
		Region: testRegion,
	}
	defaultLinodegoBucketAccess = &linodego.ObjectStorageBucketAccess{
		ACL:         linodego.ACLPrivate,
		CorsEnabled: provisioner.ParamCORSValueDisabled.Bool(),
	}

	defaultBucketParameters = map[string]string{
		provisioner.ParamRegion: testRegion,
	}

	defaultBucketAccessParameters = map[string]string{
		provisioner.ParamACL:  string(linodego.ACLPrivate),
		provisioner.ParamCORS: string(provisioner.ParamCORSValueDisabled),
	}

	defaultBucketInfo = &cosi.Protocol{
		Type: &cosi.Protocol_S3{
			S3: &cosi.S3{
				Region:           testRegion,
				SignatureVersion: cosi.S3SignatureVersion_S3V4,
			},
		},
	}
	defaultCredentials = map[string]*cosi.CredentialDetails{
		provisioner.S3: {
			Secrets: map[string]string{
				provisioner.S3Region:                testRegion,
				provisioner.S3Endpoint:              fmt.Sprintf("%s.linodeobjects.com", testBucketName),
				provisioner.S3SecretAccessKeyID:     stubclient.TestAccessKey,
				provisioner.S3SecretAccessSecretKey: stubclient.TestSecretKey,
			},
		},
	}
)

func TestDriverCreateBucket(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName         string
		client           linodeclient.Client
		request          *cosi.DriverCreateBucketRequest
		expectedResponse *cosi.DriverCreateBucketResponse
		expectedError    error
	}{
		{
			testName: "base",
			client:   stubclient.New(),
			request: &cosi.DriverCreateBucketRequest{
				Name:       testBucketName,
				Parameters: defaultBucketParameters,
			},
			expectedResponse: &cosi.DriverCreateBucketResponse{
				BucketId:   testBucketID,
				BucketInfo: defaultBucketInfo,
			},
		},
		{
			testName: "bucket exists",
			client: stubclient.New(
				stubclient.WithBucket(defaultLinodegoBucket),
				stubclient.WithBucketAccess(defaultLinodegoBucketAccess, defaultLinodegoBucket.Region, defaultLinodegoBucket.Label),
			),
			request: &cosi.DriverCreateBucketRequest{
				Name:       testBucketName,
				Parameters: defaultBucketParameters,
			},
			expectedResponse: &cosi.DriverCreateBucketResponse{
				BucketId:   testBucketID,
				BucketInfo: defaultBucketInfo,
			},
		},
		{
			testName: "empty map",
			client:   stubclient.New(),
			request: &cosi.DriverCreateBucketRequest{
				Name:       testBucketName,
				Parameters: map[string]string{},
			},
			expectedError: status.Error(grpccodes.InvalidArgument, provisioner.ErrMissingRegion.Error()),
		},
		{
			testName: "nil map",
			client:   stubclient.New(),
			request: &cosi.DriverCreateBucketRequest{
				Name: testBucketName,
			},
			expectedError: status.Error(grpccodes.InvalidArgument, provisioner.ErrMissingRegion.Error()),
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testutils.ContextFromT(context.Background(), t)
			defer cancel()

			srv, err := provisioner.New(nil, tc.client)
			if err != nil {
				t.Fatalf("failed to create provisioner server: %v", err)
			}

			for i := 0; i < 2; i++ { // run twice to check idempotency
				actual, err := srv.DriverCreateBucket(ctx, tc.request)
				if !errors.Is(err, tc.expectedError) {
					t.Errorf("call %d: expected error: %q, but got: %q", i, tc.expectedError, err)
				}

				if !reflect.DeepEqual(tc.expectedResponse, actual) {
					t.Errorf("call %d: expected credentials to be deeply equal\n> expected: %#+v,\n> got: %#+v",
						i,
						tc.expectedResponse,
						actual)
				}
			}
		})
	}
}

func TestDriverDeleteBucket(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		client        linodeclient.Client
		request       *cosi.DriverDeleteBucketRequest
		expectedError error
	}{
		{
			testName: "base",
			client:   stubclient.New(stubclient.WithBucket(defaultLinodegoBucket)),
			request: &cosi.DriverDeleteBucketRequest{
				BucketId: testBucketID,
			},
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testutils.ContextFromT(context.Background(), t)
			defer cancel()

			srv, err := provisioner.New(nil, tc.client)
			if err != nil {
				t.Fatalf("failed to create provisioner server: %v", err)
			}

			for i := 0; i < 2; i++ { // run twice to check idempotency
				_, err = srv.DriverDeleteBucket(ctx, tc.request)
				if !errors.Is(err, tc.expectedError) {
					t.Errorf("call %d: expected error: %q, but got: %q", i, tc.expectedError, err)
				}
			}
		})
	}
}

func TestDriverGrantBucketAccess(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName         string
		client           linodeclient.Client
		request          *cosi.DriverGrantBucketAccessRequest
		expectedResponse *cosi.DriverGrantBucketAccessResponse
		expectedError    error
	}{
		{
			testName: "base",
			client:   stubclient.New(stubclient.WithBucket(defaultLinodegoBucket)),
			request: &cosi.DriverGrantBucketAccessRequest{
				BucketId:           testBucketID,
				Name:               testBucketAccessName,
				AuthenticationType: cosi.AuthenticationType_Key,
				Parameters:         defaultBucketAccessParameters,
			},
			expectedResponse: &cosi.DriverGrantBucketAccessResponse{
				AccountId:   testBucketAccessID,
				Credentials: defaultCredentials,
			},
		},
		{
			testName: "IAM Auth",
			client:   stubclient.New(stubclient.WithBucket(defaultLinodegoBucket)),
			request: &cosi.DriverGrantBucketAccessRequest{
				BucketId:           testBucketID,
				Name:               testBucketAccessName,
				AuthenticationType: cosi.AuthenticationType_IAM,
				Parameters:         defaultBucketAccessParameters,
			},
			expectedError: status.Error(
				grpccodes.InvalidArgument,
				fmt.Errorf("%w: %s", provisioner.ErrUnsuportedAuth, cosi.AuthenticationType_IAM).Error(),
			),
		},
		{
			testName: "invalid permissions",
			client:   stubclient.New(stubclient.WithBucket(defaultLinodegoBucket)),
			request: &cosi.DriverGrantBucketAccessRequest{
				BucketId:           testBucketID,
				Name:               testBucketAccessName,
				AuthenticationType: cosi.AuthenticationType_Key,
				Parameters: map[string]string{
					provisioner.ParamPermissions: "invalid",
				},
			},
			expectedError: status.Error(
				grpccodes.InvalidArgument,
				fmt.Errorf("%w: %s", provisioner.ErrUnknownPermsissions, "invalid").Error(),
			),
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testutils.ContextFromT(context.Background(), t)
			defer cancel()

			srv, err := provisioner.New(nil, tc.client)
			if err != nil {
				t.Fatalf("failed to create provisioner server: %v", err)
			}

			for i := 0; i < 2; i++ { // run twice to check idempotency
				actual, err := srv.DriverGrantBucketAccess(ctx, tc.request)
				if !errors.Is(err, tc.expectedError) {
					t.Errorf("call %d: expected error: %q, but got: %q", i, tc.expectedError, err)
				}

				if !reflect.DeepEqual(tc.expectedResponse, actual) {
					t.Errorf("call %d: expected buckets to be deeply equal\n> expected: %#+v,\n> got: %#+v",
						i,
						tc.expectedResponse,
						actual)
				}
			}
		})
	}
}

func TestDriverRevokeBucketAccess(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		client        linodeclient.Client
		request       *cosi.DriverRevokeBucketAccessRequest
		expectedError error
	}{
		{
			testName: "base",
			client: stubclient.New(
				stubclient.WithBucket(defaultLinodegoBucket),
				stubclient.WithBucketAccess(defaultLinodegoBucketAccess, defaultLinodegoBucket.Region, defaultLinodegoBucket.Label),
			),
			request: &cosi.DriverRevokeBucketAccessRequest{
				BucketId:  testBucketID,
				AccountId: testBucketAccessID,
			},
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testutils.ContextFromT(context.Background(), t)
			defer cancel()

			srv, err := provisioner.New(nil, tc.client)
			if err != nil {
				t.Fatalf("failed to create provisioner server: %v", err)
			}

			for i := 0; i < 2; i++ { // run twice to check idempotency
				_, err = srv.DriverRevokeBucketAccess(ctx, tc.request)
				if !errors.Is(err, tc.expectedError) {
					t.Errorf("call %d: expected error: %q, but got: %q", i, tc.expectedError, err)
				}
			}
		})
	}
}
