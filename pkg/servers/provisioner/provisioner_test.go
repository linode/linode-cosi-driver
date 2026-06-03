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

package provisioner_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"testing"

	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient/cache"
	"github.com/linode/linode-cosi-driver/pkg/s3"
	"github.com/linode/linode-cosi-driver/pkg/servers/provisioner"
	"github.com/linode/linode-cosi-driver/testing/mock"
)

const (
	testBucketPolicyTemplate = `{
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
	testBucketPolicy = `{
	"Version":"2012-10-17",
	"Statement":[
		{
			"Effect":"Allow",
			"Action":"*",
			"Resource":[
			"arn:aws:s3:::test-bucket",
			"arn:aws:s3:::test-bucket/*"
			],
			"Principal":"*"
		}
	]
}`
	testRegion           = "test-region"
	testBucketName       = "test-bucket"
	testBucketID         = testRegion + "/" + testBucketName
	testBucketAccessName = "test-bucket-access"
	testBucketAccessID   = "0"
	testAccessKey        = "TEST_ACCESS_KEY"
	testSecretKey        = "TEST_SECRET_KEY"
)

var (
	discardLog     = slog.New(slog.DiscardHandler)
	testEndpoint   = "test-region-1.linodeobjects.com"
	testEndpointE1 = "test-region-2.linodeobjects.com"
	testEndpointE3 = "test-region-3.linodeobjects.com"

	defaultLinodegoBucket = &linodego.ObjectStorageBucket{
		Label:        testBucketName,
		Region:       testRegion,
		EndpointType: linodego.ObjectStorageEndpointE0,
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

	defaultLinodegoEndpoint = linodego.ObjectStorageEndpoint{
		Region:       testRegion,
		S3Endpoint:   &testEndpoint,
		EndpointType: linodego.ObjectStorageEndpointE0,
	}
	defaultLinodegoEndpointE1 = linodego.ObjectStorageEndpoint{
		Region:       testRegion,
		S3Endpoint:   &testEndpointE1,
		EndpointType: linodego.ObjectStorageEndpointE1,
	}
	defaultLinodegoEndpointE3 = linodego.ObjectStorageEndpoint{
		Region:       testRegion,
		S3Endpoint:   &testEndpointE3,
		EndpointType: linodego.ObjectStorageEndpointE3,
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
				provisioner.S3Endpoint:              testEndpoint,
				provisioner.S3SecretAccessKeyID:     testAccessKey,
				provisioner.S3SecretAccessSecretKey: testSecretKey,
			},
		},
	}
)

func credentialsWithEndpoint(endpoint string) map[string]*cosi.CredentialDetails {
	return map[string]*cosi.CredentialDetails{
		provisioner.S3: {
			Secrets: map[string]string{
				provisioner.S3Region:                testRegion,
				provisioner.S3Endpoint:              endpoint,
				provisioner.S3SecretAccessKeyID:     testAccessKey,
				provisioner.S3SecretAccessSecretKey: testSecretKey,
			},
		},
	}
}

func expectGetBucket(t *testing.T, mockLinode *mock.MockLinodeClient, endpointType linodego.ObjectStorageEndpointType) {
	t.Helper()

	mockLinode.EXPECT().
		GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
		Return(&linodego.ObjectStorageBucket{
			Label:        testBucketName,
			Region:       testRegion,
			EndpointType: endpointType,
		}, nil).
		Times(2)
}

func TestDriverCreateBucket(t *testing.T) {
	t.Parallel()

	const testPolicyTemplate = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::{{ .BucketName }}/*"
    }
  ]
}`

	testPolicyRendered := `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::test-bucket/*"
    }
  ]
}`

	for _, tc := range []struct {
		testName         string
		request          *cosi.DriverCreateBucketRequest
		expectedResponse *cosi.DriverCreateBucketResponse
		expectedError    error
		setupMockS3      func(*testing.T) s3.Client
		setupMockLinode  func(*testing.T) linodeclient.Client
	}{
		{
			testName: "base",
			request: &cosi.DriverCreateBucketRequest{
				Name:       testBucketName,
				Parameters: defaultBucketParameters,
			},
			expectedResponse: &cosi.DriverCreateBucketResponse{
				BucketId:   testBucketID,
				BucketInfo: defaultBucketInfo,
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - no policy provided
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				// First call: GetObjectStorageBucket returns NotFound (bucket doesn't exist)
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(nil, linodego.Error{Code: http.StatusNotFound}).
					Times(1)
				// Second call: CreateObjectStorageBucket creates the bucket
				mockLinode.EXPECT().
					CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, opts linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error) {
						if opts.EndpointType != linodego.ObjectStorageEndpointE0 {
							t.Errorf("expected endpoint type %s, got %s", linodego.ObjectStorageEndpointE0, opts.EndpointType)
						}
						return defaultLinodegoBucket, nil
					}).
					Times(1)
				// Third call (idempotency): GetObjectStorageBucket returns the bucket
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(defaultLinodegoBucket, nil).
					Times(1)
				// Fourth call (idempotency): GetObjectStorageBucketAccess validates parameters
				mockLinode.EXPECT().
					GetObjectStorageBucketAccess(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(defaultLinodegoBucketAccess, nil).
					Times(1)
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "with endpoint type",
			request: &cosi.DriverCreateBucketRequest{
				Name: testBucketName,
				Parameters: map[string]string{
					provisioner.ParamRegion:       testRegion,
					provisioner.ParamEndpointType: string(linodego.ObjectStorageEndpointE1),
				},
			},
			expectedResponse: &cosi.DriverCreateBucketResponse{
				BucketId:   testBucketID,
				BucketInfo: defaultBucketInfo,
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - no policy provided
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				e1Bucket := &linodego.ObjectStorageBucket{
					Label:        testBucketName,
					Region:       testRegion,
					EndpointType: linodego.ObjectStorageEndpointE1,
				}
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(nil, linodego.Error{Code: http.StatusNotFound}).
					Times(1)
				mockLinode.EXPECT().
					CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, opts linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error) {
						if opts.EndpointType != linodego.ObjectStorageEndpointE1 {
							t.Errorf("expected endpoint type %s, got %s", linodego.ObjectStorageEndpointE1, opts.EndpointType)
						}
						return e1Bucket, nil
					}).
					Times(1)
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(e1Bucket, nil).
					Times(1)
				mockLinode.EXPECT().
					GetObjectStorageBucketAccess(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(defaultLinodegoBucketAccess, nil).
					Times(1)
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint, defaultLinodegoEndpointE1}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "with endpoint type preference and CORS skips unsupported endpoint",
			request: &cosi.DriverCreateBucketRequest{
				Name: testBucketName,
				Parameters: map[string]string{
					provisioner.ParamRegion:                 testRegion,
					provisioner.ParamCORS:                   string(provisioner.ParamCORSValueEnabled),
					provisioner.ParamEndpointTypePreference: string(linodego.ObjectStorageEndpointE3) + "," + string(linodego.ObjectStorageEndpointE1),
				},
			},
			expectedResponse: &cosi.DriverCreateBucketResponse{
				BucketId:   testBucketID,
				BucketInfo: defaultBucketInfo,
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - no policy provided
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				e1Bucket := &linodego.ObjectStorageBucket{
					Label:        testBucketName,
					Region:       testRegion,
					EndpointType: linodego.ObjectStorageEndpointE1,
				}
				corsEnabledAccess := &linodego.ObjectStorageBucketAccess{
					ACL:         linodego.ACLPrivate,
					CorsEnabled: true,
				}
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(nil, linodego.Error{Code: http.StatusNotFound}).
					Times(1)
				mockLinode.EXPECT().
					CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, opts linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error) {
						if opts.EndpointType != linodego.ObjectStorageEndpointE1 {
							t.Errorf("expected endpoint type %s, got %s", linodego.ObjectStorageEndpointE1, opts.EndpointType)
						}
						if opts.CorsEnabled == nil || !*opts.CorsEnabled {
							t.Errorf("expected cors_enabled to be true")
						}
						return e1Bucket, nil
					}).
					Times(1)
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(e1Bucket, nil).
					Times(1)
				mockLinode.EXPECT().
					GetObjectStorageBucketAccess(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(corsEnabledAccess, nil).
					Times(1)
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{
						defaultLinodegoEndpointE3,
						defaultLinodegoEndpointE1,
					}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "with explicit CORS unsupported endpoint type",
			request: &cosi.DriverCreateBucketRequest{
				Name: testBucketName,
				Parameters: map[string]string{
					provisioner.ParamRegion:       testRegion,
					provisioner.ParamCORS:         string(provisioner.ParamCORSValueEnabled),
					provisioner.ParamEndpointType: string(linodego.ObjectStorageEndpointE3),
				},
			},
			expectedError: status.Error(grpccodes.InvalidArgument, "endpoint type E3 does not support CORS"),
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - endpoint selection fails first
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{
						defaultLinodegoEndpointE3,
						defaultLinodegoEndpointE1,
					}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "bucket exists",
			request: &cosi.DriverCreateBucketRequest{
				Name:       testBucketName,
				Parameters: defaultBucketParameters,
			},
			expectedResponse: &cosi.DriverCreateBucketResponse{
				BucketId:   testBucketID,
				BucketInfo: defaultBucketInfo,
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - no policy provided
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				// Both calls: GetObjectStorageBucket returns the existing bucket
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(defaultLinodegoBucket, nil).
					Times(2)
				// Both calls: GetObjectStorageBucketAccess validates parameters
				mockLinode.EXPECT().
					GetObjectStorageBucketAccess(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(defaultLinodegoBucketAccess, nil).
					Times(2)
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "with policy template",
			request: &cosi.DriverCreateBucketRequest{
				Name: testBucketName,
				Parameters: map[string]string{
					provisioner.ParamRegion: testRegion,
					provisioner.ParamPolicy: testPolicyTemplate,
				},
			},
			expectedResponse: &cosi.DriverCreateBucketResponse{
				BucketId:   testBucketID,
				BucketInfo: defaultBucketInfo,
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// Expect SetBucketPolicy to be called twice (idempotency test runs twice)
				mockS3.EXPECT().
					SetBucketPolicy(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName), gomock.Eq(testPolicyRendered)).
					Return(nil).
					Times(2)
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				// First call: GetObjectStorageBucket returns NotFound (bucket doesn't exist)
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(nil, linodego.Error{Code: http.StatusNotFound}).
					Times(1)
				// Second call: CreateObjectStorageBucket creates the bucket
				mockLinode.EXPECT().
					CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
					Return(defaultLinodegoBucket, nil).
					Times(1)
				// Third call (idempotency): GetObjectStorageBucket returns the bucket
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(defaultLinodegoBucket, nil).
					Times(1)
				// Fourth call (idempotency): GetObjectStorageBucketAccess validates parameters
				mockLinode.EXPECT().
					GetObjectStorageBucketAccess(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(defaultLinodegoBucketAccess, nil).
					Times(1)
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "with policy template, bucket exists",
			request: &cosi.DriverCreateBucketRequest{
				Name: testBucketName,
				Parameters: map[string]string{
					provisioner.ParamRegion: testRegion,
					provisioner.ParamPolicy: testPolicyTemplate,
				},
			},
			expectedResponse: &cosi.DriverCreateBucketResponse{
				BucketId:   testBucketID,
				BucketInfo: defaultBucketInfo,
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// Expect SetBucketPolicy to be called twice (for existing buckets, policy is always applied)
				mockS3.EXPECT().
					SetBucketPolicy(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName), gomock.Eq(testPolicyRendered)).
					Return(nil).
					Times(2)
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				// Both calls: GetObjectStorageBucket returns the existing bucket
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(defaultLinodegoBucket, nil).
					Times(2)
				// Both calls: GetObjectStorageBucketAccess validates parameters
				mockLinode.EXPECT().
					GetObjectStorageBucketAccess(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(defaultLinodegoBucketAccess, nil).
					Times(2)
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "SetBucketPolicy fails",
			request: &cosi.DriverCreateBucketRequest{
				Name: testBucketName,
				Parameters: map[string]string{
					provisioner.ParamRegion: testRegion,
					provisioner.ParamPolicy: testPolicyTemplate,
				},
			},
			expectedError: status.Error(grpccodes.Internal, "failed to set bucket policy: S3 connection failed"),
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// Expect SetBucketPolicy to fail on both calls (idempotency test runs twice)
				// First call creates bucket, second call sees existing bucket - both try to apply policy
				mockS3.EXPECT().
					SetBucketPolicy(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(errors.New("S3 connection failed")).
					Times(2)
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				// First call: GetObjectStorageBucket returns NotFound (bucket doesn't exist)
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(nil, linodego.Error{Code: http.StatusNotFound}).
					Times(1)
				// Second call: CreateObjectStorageBucket creates the bucket
				mockLinode.EXPECT().
					CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
					Return(defaultLinodegoBucket, nil).
					Times(1)
				// Third call (idempotency): GetObjectStorageBucket returns the bucket
				mockLinode.EXPECT().
					GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(defaultLinodegoBucket, nil).
					Times(1)
				// Fourth call (idempotency): GetObjectStorageBucketAccess validates parameters
				mockLinode.EXPECT().
					GetObjectStorageBucketAccess(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(defaultLinodegoBucketAccess, nil).
					Times(1)
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "empty map",
			request: &cosi.DriverCreateBucketRequest{
				Name:       testBucketName,
				Parameters: map[string]string{},
			},
			expectedError: status.Error(grpccodes.InvalidArgument, provisioner.ErrMissingRegion.Error()),
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - validation fails before S3 operations
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				// No Linode calls expected - validation fails before any API operations
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "nil map",
			request: &cosi.DriverCreateBucketRequest{
				Name: testBucketName,
			},
			expectedError: status.Error(grpccodes.InvalidArgument, provisioner.ErrMissingRegion.Error()),
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - validation fails before S3 operations
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				// No Linode calls expected - validation fails before any API operations
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			linodeCli := tc.setupMockLinode(t)
			epc := cache.New(discardLog, linodeCli, 0)
			if err := epc.Refresh(ctx); err != nil {
				t.Fatalf("failed to refresh cache: %v", err)
			}

			s3cli := tc.setupMockS3(t)

			srv, err := provisioner.New(nil, linodeCli, epc, s3cli, true)
			if err != nil {
				t.Fatalf("failed to create provisioner server: %v", err)
			}

			for i := 0; i < 2; i++ { //nolint:varnamelen //simple loop, run twice to check for idepotency
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

func TestDriverCreateBucketWithPolicyTemplate(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// Setup mock S3 client with stateful behavior
	ctrl := gomock.NewController(t)
	mockS3 := mock.NewMockS3Client(ctrl)

	// Variable to store the policy (simulates S3 storage)
	var storedPolicy string

	// SetBucketPolicy stores the policy
	mockS3.EXPECT().
		SetBucketPolicy(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName), gomock.Any()).
		DoAndReturn(func(ctx context.Context, region, bucketName, policy string) error {
			storedPolicy = policy
			return nil
		}).
		Times(2) // Called twice due to idempotency test

	// GetBucketPolicy retrieves the stored policy
	mockS3.EXPECT().
		GetBucketPolicy(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
		DoAndReturn(func(ctx context.Context, region, bucketName string) (string, error) {
			return storedPolicy, nil
		}).
		Times(1) // Called once at the end

	// Setup mock Linode client
	mockLinode := mock.NewMockLinodeClient(ctrl)

	// First call: GetObjectStorageBucket returns NotFound (bucket doesn't exist)
	mockLinode.EXPECT().
		GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
		Return(nil, linodego.Error{Code: http.StatusNotFound}).
		Times(1)

	// Second call: CreateObjectStorageBucket creates the bucket
	mockLinode.EXPECT().
		CreateObjectStorageBucket(gomock.Any(), gomock.Any()).
		Return(defaultLinodegoBucket, nil).
		Times(1)

	// Third call (idempotency): GetObjectStorageBucket returns the bucket
	mockLinode.EXPECT().
		GetObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
		Return(defaultLinodegoBucket, nil).
		Times(1)

	// Fourth call (idempotency): GetObjectStorageBucketAccess validates parameters
	mockLinode.EXPECT().
		GetObjectStorageBucketAccess(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
		Return(defaultLinodegoBucketAccess, nil).
		Times(1)

	// ListObjectStorageEndpoints is called to populate cache
	mockLinode.EXPECT().
		ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
		Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
		AnyTimes()

	// Create cache and provisioner
	epc := cache.New(discardLog, mockLinode, 0)
	if err := epc.Refresh(ctx); err != nil {
		t.Fatalf("failed to refresh cache: %v", err)
	}

	srv, err := provisioner.New(nil, mockLinode, epc, mockS3, true)
	if err != nil {
		t.Fatalf("failed to create provisioner server: %v", err)
	}

	req := &cosi.DriverCreateBucketRequest{
		Name: testBucketName,
		Parameters: map[string]string{
			provisioner.ParamRegion: testRegion,
			provisioner.ParamPolicy: testBucketPolicyTemplate,
		},
	}

	expected := &cosi.DriverCreateBucketResponse{
		BucketId:   testBucketID,
		BucketInfo: defaultBucketInfo,
	}

	// Run twice to test idempotency
	for callIndex := 0; callIndex < 2; callIndex++ {
		actual, err := srv.DriverCreateBucket(ctx, req)
		if err != nil {
			t.Fatalf("call %d: expected no error, but got: %v", callIndex, err)
		}

		if !reflect.DeepEqual(expected, actual) {
			t.Fatalf("call %d: expected response %#+v, got %#+v", callIndex, expected, actual)
		}
	}

	// Verify the policy was set correctly by retrieving it
	actualPolicy, err := mockS3.GetBucketPolicy(ctx, testRegion, testBucketName)
	if err != nil {
		t.Fatalf("expected bucket policy to be set, got error: %v", err)
	}

	if actualPolicy != testBucketPolicy {
		t.Fatalf("expected policy %q, got %q", testBucketPolicy, actualPolicy)
	}
}

func TestDriverDeleteBucket(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName        string
		request         *cosi.DriverDeleteBucketRequest
		expectedError   error
		setupMockS3     func(*testing.T) s3.Client
		setupMockLinode func(*testing.T) linodeclient.Client
	}{
		{
			testName: "base",
			request: &cosi.DriverDeleteBucketRequest{
				BucketId: testBucketID,
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - cleanup is disabled (hardcoded to false in provisioner.go:278)
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				// Both calls: DeleteObjectStorageBucket deletes the bucket
				mockLinode.EXPECT().
					DeleteObjectStorageBucket(gomock.Any(), gomock.Eq(testRegion), gomock.Eq(testBucketName)).
					Return(nil).
					Times(2)
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			linodeCli := tc.setupMockLinode(t)
			epc := cache.New(discardLog, linodeCli, 0)
			if err := epc.Refresh(ctx); err != nil {
				t.Fatalf("failed to refresh cache: %v", err)
			}

			s3cli := tc.setupMockS3(t)

			srv, err := provisioner.New(nil, linodeCli, epc, s3cli, true)
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
		request          *cosi.DriverGrantBucketAccessRequest
		expectedResponse *cosi.DriverGrantBucketAccessResponse
		expectedError    error
		setupMockS3      func(*testing.T) s3.Client
		setupMockLinode  func(*testing.T) linodeclient.Client
	}{
		{
			testName: "base",
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
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - GrantBucketAccess only uses Linode API
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				expectGetBucket(t, mockLinode, linodego.ObjectStorageEndpointE0)
				// Both calls: CreateObjectStorageKey creates the key
				mockLinode.EXPECT().
					CreateObjectStorageKey(gomock.Any(), gomock.Any()).
					Return(&linodego.ObjectStorageKey{
						ID:        0,
						AccessKey: testAccessKey,
						SecretKey: testSecretKey,
					}, nil).
					Times(2)
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "uses bucket endpoint type despite access endpoint type parameter",
			request: &cosi.DriverGrantBucketAccessRequest{
				BucketId:           testBucketID,
				Name:               testBucketAccessName,
				AuthenticationType: cosi.AuthenticationType_Key,
				Parameters: map[string]string{
					provisioner.ParamPermissions:  string(provisioner.ParamPermissionsValueReadOnly),
					provisioner.ParamEndpointType: string(linodego.ObjectStorageEndpointE1),
				},
			},
			expectedResponse: &cosi.DriverGrantBucketAccessResponse{
				AccountId:   testBucketAccessID,
				Credentials: defaultCredentials,
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - GrantBucketAccess only uses Linode API
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				expectGetBucket(t, mockLinode, linodego.ObjectStorageEndpointE0)
				mockLinode.EXPECT().
					CreateObjectStorageKey(gomock.Any(), gomock.Any()).
					Return(&linodego.ObjectStorageKey{
						ID:        0,
						AccessKey: testAccessKey,
						SecretKey: testSecretKey,
					}, nil).
					Times(2)
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint, defaultLinodegoEndpointE1}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "with endpoint type preference fallback",
			request: &cosi.DriverGrantBucketAccessRequest{
				BucketId:           testBucketID,
				Name:               testBucketAccessName,
				AuthenticationType: cosi.AuthenticationType_Key,
				Parameters: map[string]string{
					provisioner.ParamPermissions:            string(provisioner.ParamPermissionsValueReadOnly),
					provisioner.ParamEndpointTypePreference: string(linodego.ObjectStorageEndpointE3) + "," + string(linodego.ObjectStorageEndpointE1),
				},
			},
			expectedResponse: &cosi.DriverGrantBucketAccessResponse{
				AccountId:   testBucketAccessID,
				Credentials: credentialsWithEndpoint(testEndpointE1),
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - GrantBucketAccess only uses Linode API
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				expectGetBucket(t, mockLinode, linodego.ObjectStorageEndpointE1)
				mockLinode.EXPECT().
					CreateObjectStorageKey(gomock.Any(), gomock.Any()).
					Return(&linodego.ObjectStorageKey{
						ID:        0,
						AccessKey: testAccessKey,
						SecretKey: testSecretKey,
					}, nil).
					Times(2)
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint, defaultLinodegoEndpointE1}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "uses bucket endpoint type despite unavailable access preference",
			request: &cosi.DriverGrantBucketAccessRequest{
				BucketId:           testBucketID,
				Name:               testBucketAccessName,
				AuthenticationType: cosi.AuthenticationType_Key,
				Parameters: map[string]string{
					provisioner.ParamPermissions:            string(provisioner.ParamPermissionsValueReadOnly),
					provisioner.ParamEndpointTypePreference: string(linodego.ObjectStorageEndpointE3),
				},
			},
			expectedResponse: &cosi.DriverGrantBucketAccessResponse{
				AccountId:   testBucketAccessID,
				Credentials: defaultCredentials,
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - endpoint selection fails first
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				expectGetBucket(t, mockLinode, linodego.ObjectStorageEndpointE0)
				mockLinode.EXPECT().
					CreateObjectStorageKey(gomock.Any(), gomock.Any()).
					Return(&linodego.ObjectStorageKey{
						ID:        0,
						AccessKey: testAccessKey,
						SecretKey: testSecretKey,
					}, nil).
					Times(2)
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint, defaultLinodegoEndpointE3}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "default endpoint type from endpoints list",
			request: &cosi.DriverGrantBucketAccessRequest{
				BucketId:           testBucketID,
				Name:               testBucketAccessName,
				AuthenticationType: cosi.AuthenticationType_Key,
				Parameters:         defaultBucketAccessParameters,
			},
			expectedResponse: &cosi.DriverGrantBucketAccessResponse{
				AccountId:   testBucketAccessID,
				Credentials: credentialsWithEndpoint(testEndpointE1),
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - GrantBucketAccess only uses Linode API
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				expectGetBucket(t, mockLinode, linodego.ObjectStorageEndpointE1)
				mockLinode.EXPECT().
					CreateObjectStorageKey(gomock.Any(), gomock.Any()).
					Return(&linodego.ObjectStorageKey{
						ID:        0,
						AccessKey: testAccessKey,
						SecretKey: testSecretKey,
					}, nil).
					Times(2)
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpointE1}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "IAM Auth",
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
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - validation fails before any operations
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				// No Linode calls expected - validation fails before any API operations
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
		{
			testName: "invalid permissions",
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
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - validation fails before any operations
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				// No Linode calls expected - validation fails before any API operations
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			linodeCli := tc.setupMockLinode(t)
			epc := cache.New(discardLog, linodeCli, 0)
			if err := epc.Refresh(ctx); err != nil {
				t.Fatalf("failed to refresh cache: %v", err)
			}

			s3cli := tc.setupMockS3(t)

			srv, err := provisioner.New(nil, linodeCli, epc, s3cli, true)
			if err != nil {
				t.Fatalf("failed to create provisioner server: %v", err)
			}

			for i := 0; i < 2; i++ { //nolint:varnamelen //simple loop, run twice to check for idepotency
				actual, err := srv.DriverGrantBucketAccess(ctx, tc.request)
				if !errors.Is(err, tc.expectedError) {
					t.Errorf("call %d: expected error: %q, but got: %q", i, tc.expectedError, err)
				}

				if !reflect.DeepEqual(tc.expectedResponse, actual) {
					t.Errorf("call %d: expected accesses to be deeply equal\n> expected: %#+v,\n> got: %#+v",
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
		testName        string
		request         *cosi.DriverRevokeBucketAccessRequest
		expectedError   error
		setupMockS3     func(*testing.T) s3.Client
		setupMockLinode func(*testing.T) linodeclient.Client
	}{
		{
			testName: "base",
			request: &cosi.DriverRevokeBucketAccessRequest{
				BucketId:  testBucketID,
				AccountId: testBucketAccessID,
			},
			setupMockS3: func(t *testing.T) s3.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockS3 := mock.NewMockS3Client(ctrl)
				// No S3 calls expected - RevokeBucketAccess only uses Linode API
				return mockS3
			},
			setupMockLinode: func(t *testing.T) linodeclient.Client {
				t.Helper()
				ctrl := gomock.NewController(t)
				mockLinode := mock.NewMockLinodeClient(ctrl)
				// Both calls: DeleteObjectStorageKey deletes the key
				mockLinode.EXPECT().
					DeleteObjectStorageKey(gomock.Any(), gomock.Eq(0)).
					Return(nil).
					Times(2)
				// ListObjectStorageEndpoints is called to populate cache
				mockLinode.EXPECT().
					ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
					Return([]linodego.ObjectStorageEndpoint{defaultLinodegoEndpoint}, nil).
					AnyTimes()
				return mockLinode
			},
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			linodeCli := tc.setupMockLinode(t)
			epc := cache.New(discardLog, linodeCli, 0)
			if err := epc.Refresh(ctx); err != nil {
				t.Fatalf("failed to refresh cache: %v", err)
			}

			s3cli := tc.setupMockS3(t)

			srv, err := provisioner.New(nil, linodeCli, epc, s3cli, true)
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
