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
	testRegion               = "test-region"
	testBucketName           = "test-bucket"
	testBucketID             = testRegion + "/" + testBucketName
	testBucketAccessName     = "test-bucket-access"
	testBucketAccessID       = "0"
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
	discardLog   = slog.New(slog.DiscardHandler)
	testEndpoint = "test-region-1.linodeobjects.com"

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

	defaultLinodegoEndpoint = linodego.ObjectStorageEndpoint{
		Region:       testRegion,
		S3Endpoint:   &testEndpoint,
		EndpointType: linodego.ObjectStorageEndpointE0,
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

	client := linodestub.New()
	s3cli := s3stub.New()
	epc := cache.New(discardLog, client, 0)
	if err := epc.Refresh(ctx); err != nil {
		t.Fatalf("failed to refresh cache: %v", err)
	}

	s3stub.SetBucketTracker(s3cli, client)

	srv, err := provisioner.New(nil, client, epc, s3cli, true)
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

	for callIndex := 0; callIndex < 2; callIndex++ {
		actual, err := srv.DriverCreateBucket(ctx, req)
		if err != nil {
			t.Fatalf("call %d: expected no error, but got: %v", callIndex, err)
		}

		if !reflect.DeepEqual(expected, actual) {
			t.Fatalf("call %d: expected response %#+v, got %#+v", callIndex, expected, actual)
		}
	}

	actualPolicy, err := s3cli.GetBucketPolicy(ctx, testRegion, testBucketName)
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
