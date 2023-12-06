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

package stubclient_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient/stubclient"
	"github.com/linode/linode-cosi-driver/pkg/testutils"
	"github.com/linode/linodego"
)

var (
	testBucket = &linodego.ObjectStorageBucket{
		Label:    "test-label",
		Cluster:  "test-cluster",
		Hostname: "test-label.linodeobjects.com",
		Objects:  0,
		Size:     0,
	}

	testBucketAccess = &linodego.ObjectStorageBucketAccess{
		ACL:         linodego.ACLPrivate,
		CorsEnabled: false,
	}

	testKeyBucketAccessList = []linodego.ObjectStorageKeyBucketAccess{
		{
			Cluster:     "test-cluster",
			BucketName:  "test-label",
			Permissions: "test-permissions",
		},
	}

	testKey = &linodego.ObjectStorageKey{
		ID:        0,
		Label:     "test",
		AccessKey: stubclient.TestAccessKey,
		SecretKey: stubclient.TestSecretKey,
	}

	test500Error = &linodego.Error{
		Code: http.StatusInternalServerError,
	}

	testBool = true
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			testName string // required
			input    []stubclient.Option
		}{
			{
				testName: "nil input",
			},
			{
				testName: "buckets input",
				input: []stubclient.Option{
					stubclient.WithBucket(testBucket),
				},
			},
			{
				testName: "bucket accesses input",
				input: []stubclient.Option{
					stubclient.WithBucketAccess(testBucketAccess, testBucket.Cluster, testBucket.Label),
				},
			},
			{
				testName: "keys input",
				input: []stubclient.Option{
					stubclient.WithKey(testKey),
				},
			},
			{
				testName: "mixed input",
				input: []stubclient.Option{
					stubclient.WithBucket(testBucket),
					stubclient.WithBucketAccess(testBucketAccess, testBucket.Cluster, testBucket.Label),
					stubclient.WithKey(testKey),
				},
			},
		} {
			tc := tc

			t.Run(tc.testName, func(t *testing.T) {
				t.Parallel()

				testutils.AssertNotPanics(t, func() {
					_ = stubclient.New(tc.input...)
				})
			})
		}
	})
}

func TestCreateObjectStorageBucket(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		ctx           context.Context //nolint:containedctx
		client        linodeclient.Client
		opts          linodego.ObjectStorageBucketCreateOptions
		expectedError error
		expectedValue *linodego.ObjectStorageBucket
	}{
		{
			testName: "valid input",
			client:   stubclient.New(),
			opts: linodego.ObjectStorageBucketCreateOptions{
				Cluster: testBucket.Cluster,
				Label:   testBucket.Label,
				ACL:     linodego.ACLPrivate,
			},
			expectedError: nil,
			expectedValue: testBucket,
		},
		{
			testName: "duplicated",
			client:   stubclient.New(stubclient.WithBucket(testBucket)),
			opts: linodego.ObjectStorageBucketCreateOptions{
				Cluster: testBucket.Cluster,
				Label:   testBucket.Label,
				ACL:     linodego.ACLPrivate,
			},
			expectedError: nil,
			expectedValue: testBucket,
		},
		{
			testName: "with CORS",
			client:   stubclient.New(),
			opts: linodego.ObjectStorageBucketCreateOptions{
				Cluster:     testBucket.Cluster,
				Label:       testBucket.Label,
				ACL:         linodego.ACLPrivate,
				CorsEnabled: &testBool,
			},
			expectedError: nil,
			expectedValue: testBucket,
		},
		{
			testName: "invalid ACL",
			client:   stubclient.New(),
			opts: linodego.ObjectStorageBucketCreateOptions{
				Cluster: testBucket.Cluster,
				Label:   testBucket.Label,
				ACL:     "invalid-acl",
			},
			expectedError: &linodego.Error{Code: http.StatusBadRequest},
			expectedValue: nil,
		},
		{
			testName: "unexpected failure",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				"",
			),
			expectedError: stubclient.ErrUnexpectedError,
		},
		{
			testName: "simulated internal server error",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				test500Error,
			),
			expectedError: test500Error,
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			ctx, cancel := testutils.ContextFromT(tc.ctx, t)
			defer cancel()

			actual, err := tc.client.CreateObjectStorageBucket(ctx, tc.opts)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(tc.expectedValue, actual) {
				t.Errorf("expected value to be deeply equal\n> expected: %#+v,\n> got: %#+v",
					tc.expectedValue,
					actual)
			}
		})
	}
}

func TestGetObjectStorageBucket(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		ctx           context.Context //nolint:containedctx
		client        linodeclient.Client
		clusterID     string
		label         string
		expectedValue *linodego.ObjectStorageBucket
		expectedError error
	}{
		{
			testName:      "valid input",
			client:        stubclient.New(stubclient.WithBucket(testBucket)),
			clusterID:     testBucket.Cluster,
			label:         testBucket.Label,
			expectedValue: testBucket,
			expectedError: nil,
		},
		{
			testName:      "non existent bucket",
			client:        stubclient.New(),
			clusterID:     "non-existent-cluster",
			label:         "non-existent-label",
			expectedValue: nil,
			expectedError: &linodego.Error{Code: http.StatusNotFound},
		},
		{
			testName: "unexpected failure",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				"",
			),
			expectedError: stubclient.ErrUnexpectedError,
		},
		{
			testName: "simulated internal server error",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				test500Error,
			),
			expectedError: test500Error,
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			ctx, cancel := testutils.ContextFromT(tc.ctx, t)
			defer cancel()

			actual, err := tc.client.GetObjectStorageBucket(ctx, tc.clusterID, tc.label)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(tc.expectedValue, actual) {
				t.Errorf("expected value to be deeply equal\n> expected: %#+v,\n> got: %#+v",
					tc.expectedValue,
					actual)
			}
		})
	}
}

func TestDeleteObjectStorageBucket(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		ctx           context.Context //nolint:containedctx
		client        linodeclient.Client
		clusterID     string
		label         string
		expectedError error
	}{
		{
			testName:      "valid input",
			client:        stubclient.New(stubclient.WithBucket(testBucket)),
			clusterID:     testBucket.Cluster,
			label:         testBucket.Label,
			expectedError: nil,
		},
		{
			testName:      "non existent bucket",
			client:        stubclient.New(),
			clusterID:     testBucket.Cluster,
			label:         testBucket.Label,
			expectedError: &linodego.Error{Code: http.StatusNotFound},
		},
		{
			testName: "non empty bucket",
			client: stubclient.New(stubclient.WithBucket(&linodego.ObjectStorageBucket{
				Cluster:  testBucket.Cluster,
				Label:    testBucket.Label,
				Hostname: testBucket.Hostname,
				Objects:  10,
				Size:     102310,
			})),
			clusterID:     testBucket.Cluster,
			label:         testBucket.Label,
			expectedError: &linodego.Error{Code: http.StatusBadRequest},
		},
		{
			testName: "unexpected failure",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				"",
			),
			expectedError: stubclient.ErrUnexpectedError,
		},
		{
			testName: "simulated internal server error",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				test500Error,
			),
			expectedError: test500Error,
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			ctx, cancel := testutils.ContextFromT(tc.ctx, t)
			defer cancel()

			err := tc.client.DeleteObjectStorageBucket(ctx, tc.clusterID, tc.label)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}

func TestGetObjectStorageBucketAccess(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		ctx           context.Context //nolint:containedctx
		client        linodeclient.Client
		clusterID     string
		label         string
		expectedValue *linodego.ObjectStorageBucketAccess
		expectedError error
	}{
		{
			testName: "valid input",
			client: stubclient.New(
				stubclient.WithBucketAccess(testBucketAccess, testBucket.Cluster, testBucket.Label),
			),
			clusterID:     testBucket.Cluster,
			label:         testBucket.Label,
			expectedValue: testBucketAccess,
			expectedError: nil,
		},
		{
			testName:      "non existent bucket",
			client:        stubclient.New(),
			clusterID:     "non-existent-cluster",
			label:         "non-existent-label",
			expectedValue: nil,
			expectedError: &linodego.Error{Code: http.StatusNotFound},
		},
		{
			testName: "unexpected failure",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				"",
			),
			expectedError: stubclient.ErrUnexpectedError,
		},
		{
			testName: "simulated internal server error",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				test500Error,
			),
			expectedError: test500Error,
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			ctx, cancel := testutils.ContextFromT(tc.ctx, t)
			defer cancel()

			actual, err := tc.client.GetObjectStorageBucketAccess(ctx, tc.clusterID, tc.label)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(tc.expectedValue, actual) {
				t.Errorf("expected value to be deeply equal\n> expected: %#+v,\n> got: %#+v",
					tc.expectedValue,
					actual)
			}
		})
	}
}

func TestUpdateObjectStorageBucketAccess(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		ctx           context.Context //nolint:containedctx
		client        linodeclient.Client
		clusterID     string
		label         string
		opts          linodego.ObjectStorageBucketUpdateAccessOptions
		expectedError error
	}{
		{
			testName: "valid input",
			client: stubclient.New(
				stubclient.WithBucketAccess(testBucketAccess, testBucket.Cluster, testBucket.Label),
			),
			clusterID: testBucket.Cluster,
			label:     testBucket.Label,
			opts: linodego.ObjectStorageBucketUpdateAccessOptions{
				ACL: testBucketAccess.ACL,
			},
			expectedError: nil,
		},
		{
			testName: "with CORS",
			client: stubclient.New(
				stubclient.WithBucketAccess(testBucketAccess, testBucket.Cluster, testBucket.Label),
			),
			clusterID: testBucket.Cluster,
			label:     testBucket.Label,
			opts: linodego.ObjectStorageBucketUpdateAccessOptions{
				ACL:         testBucketAccess.ACL,
				CorsEnabled: &testBool,
			},
			expectedError: nil,
		},
		{
			testName: "invalid ACL",
			client: stubclient.New(
				stubclient.WithBucketAccess(testBucketAccess, testBucket.Cluster, testBucket.Label),
			),
			clusterID: testBucket.Cluster,
			label:     testBucket.Label,
			opts: linodego.ObjectStorageBucketUpdateAccessOptions{
				ACL: "invalid-acl",
			},
			expectedError: linodego.Error{Code: http.StatusBadRequest},
		},
		{
			testName:      "non existent input",
			client:        stubclient.New(),
			clusterID:     "non-existent-cluster",
			label:         "non-existent-label",
			opts:          linodego.ObjectStorageBucketUpdateAccessOptions{},
			expectedError: linodego.Error{Code: http.StatusNotFound},
		},
		{
			testName: "unexpected failure",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				"",
			),
			expectedError: stubclient.ErrUnexpectedError,
		},
		{
			testName: "simulated internal server error",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				test500Error,
			),
			expectedError: test500Error,
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			ctx, cancel := testutils.ContextFromT(tc.ctx, t)
			defer cancel()

			err := tc.client.UpdateObjectStorageBucketAccess(ctx, tc.clusterID, tc.label, tc.opts)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}

func TestCreateObjectStorageKey(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		ctx           context.Context //nolint:containedctx
		client        linodeclient.Client
		opts          linodego.ObjectStorageKeyCreateOptions
		expectedValue *linodego.ObjectStorageKey
		expectedError error
	}{
		{
			testName: "valid input",
			client:   stubclient.New(),
			opts: linodego.ObjectStorageKeyCreateOptions{
				Label: testKey.Label,
			},
			expectedValue: testKey,
			expectedError: nil,
		},
		{
			testName: "limited key",
			client:   stubclient.New(),
			opts: linodego.ObjectStorageKeyCreateOptions{
				Label:        testKey.Label,
				BucketAccess: &testKeyBucketAccessList,
			},
			expectedValue: &linodego.ObjectStorageKey{
				ID:           0,
				Label:        "test",
				AccessKey:    stubclient.TestAccessKey,
				SecretKey:    stubclient.TestSecretKey,
				Limited:      true,
				BucketAccess: &testKeyBucketAccessList,
			},
			expectedError: nil,
		},
		{
			testName: "unexpected failure",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				"",
			),
			expectedError: stubclient.ErrUnexpectedError,
		},
		{
			testName: "simulated internal server error",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				test500Error,
			),
			expectedError: test500Error,
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			ctx, cancel := testutils.ContextFromT(tc.ctx, t)
			defer cancel()

			actual, err := tc.client.CreateObjectStorageKey(ctx, tc.opts)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(tc.expectedValue, actual) {
				t.Errorf("expected value to be deeply equal\n> expected: %#+v,\n> got: %#+v",
					tc.expectedValue,
					actual)
			}
		})
	}
}

func TestListObjectStorageKeys(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		ctx           context.Context //nolint:containedctx
		client        linodeclient.Client
		opts          *linodego.ListOptions
		expectedValue []linodego.ObjectStorageKey
		expectedError error
	}{
		{
			testName: "valid input",
			client:   stubclient.New(stubclient.WithKey(testKey)),
			opts:     &linodego.ListOptions{},
			expectedValue: []linodego.ObjectStorageKey{
				*testKey,
			},
			expectedError: nil,
		},
		{
			testName: "valid input with pagination",
			client:   stubclient.New(stubclient.WithKey(testKey)),
			opts: &linodego.ListOptions{
				PageOptions: &linodego.PageOptions{
					Page: 1,
				},
				PageSize: 10,
			},
			expectedValue: []linodego.ObjectStorageKey{
				*testKey,
			},
			expectedError: nil,
		},
		{
			testName: "valid input with invalid pagination (negative page)",
			client:   stubclient.New(stubclient.WithKey(testKey)),
			opts: &linodego.ListOptions{
				PageOptions: &linodego.PageOptions{
					Page: -1,
				},
				PageSize: 10,
			},
			expectedValue: []linodego.ObjectStorageKey{
				*testKey,
			},
			expectedError: nil,
		},
		{
			testName: "valid input with invalid pagination (negative page size)",
			client:   stubclient.New(stubclient.WithKey(testKey)),
			opts: &linodego.ListOptions{
				PageOptions: &linodego.PageOptions{
					Page: 1,
				},
				PageSize: -10,
			},
			expectedValue: []linodego.ObjectStorageKey{
				*testKey,
			},
			expectedError: nil,
		},
		{
			testName: "valid input outside of list",
			client:   stubclient.New(stubclient.WithKey(testKey)),
			opts: &linodego.ListOptions{
				PageOptions: &linodego.PageOptions{
					Page: 10,
				},
				PageSize: 10,
			},
			expectedValue: []linodego.ObjectStorageKey{},
			expectedError: nil,
		},
		{
			testName:      "valid input, empty list",
			client:        stubclient.New(),
			opts:          &linodego.ListOptions{},
			expectedValue: []linodego.ObjectStorageKey{},
			expectedError: nil,
		},
		{
			testName: "unexpected failure",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				"",
			),
			expectedError: stubclient.ErrUnexpectedError,
		},
		{
			testName: "simulated internal server error",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				test500Error,
			),
			expectedError: test500Error,
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			ctx, cancel := testutils.ContextFromT(tc.ctx, t)
			defer cancel()

			actual, err := tc.client.ListObjectStorageKeys(ctx, tc.opts)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(tc.expectedValue, actual) {
				t.Errorf("expected value to be deeply equal\n> expected: %#+v,\n> got: %#+v",
					tc.expectedValue,
					actual)
			}
		})
	}
}

func TestGetObjectStorageKey(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		ctx           context.Context //nolint:containedctx
		client        linodeclient.Client
		keyID         int
		expectedValue *linodego.ObjectStorageKey
		expectedError error
	}{
		{
			testName:      "valid input",
			client:        stubclient.New(stubclient.WithKey(testKey)),
			keyID:         testKey.ID,
			expectedValue: testKey,
			expectedError: nil,
		},
		{
			testName:      "non existent key",
			client:        stubclient.New(),
			keyID:         2001,
			expectedValue: nil,
			expectedError: &linodego.Error{Code: http.StatusNotFound},
		},
		{
			testName: "unexpected failure",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				"",
			),
			expectedError: stubclient.ErrUnexpectedError,
		},
		{
			testName: "simulated internal server error",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				test500Error,
			),
			expectedError: test500Error,
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			ctx, cancel := testutils.ContextFromT(tc.ctx, t)
			defer cancel()

			actual, err := tc.client.GetObjectStorageKey(ctx, tc.keyID)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(tc.expectedValue, actual) {
				t.Errorf("expected value to be deeply equal\n> expected: %#+v,\n> got: %#+v",
					tc.expectedValue,
					actual)
			}
		})
	}
}

func TestDeleteObjectStorageKey(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		ctx           context.Context //nolint:containedctx
		client        linodeclient.Client
		keyID         int
		expectedError error
	}{
		{
			testName:      "valid input",
			client:        stubclient.New(stubclient.WithKey(testKey)),
			keyID:         testKey.ID,
			expectedError: nil,
		},
		{
			testName:      "non existent key",
			client:        stubclient.New(),
			keyID:         2001,
			expectedError: &linodego.Error{Code: http.StatusNotFound},
		},
		{
			testName: "unexpected failure",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				"",
			),
			expectedError: stubclient.ErrUnexpectedError,
		},
		{
			testName: "simulated internal server error",
			client:   stubclient.New(),
			ctx: context.WithValue(
				context.Background(),
				stubclient.ForcedFailure, //nolint:staticcheck
				test500Error,
			),
			expectedError: test500Error,
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			if tc.ctx == nil {
				tc.ctx = context.Background()
			}

			ctx, cancel := testutils.ContextFromT(tc.ctx, t)
			defer cancel()

			err := tc.client.DeleteObjectStorageKey(ctx, tc.keyID)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}
