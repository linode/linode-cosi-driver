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
	"reflect"
	"testing"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/pkg/servers/provisioner"
	"github.com/linode/linode-cosi-driver/pkg/testutils"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"
)

func TestDriverCreateBucket(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName         string
		client           linodeclient.LinodeClient
		request          *cosi.DriverCreateBucketRequest
		expectedResponse *cosi.DriverCreateBucketResponse
		expectedError    error
	}{} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testutils.ContextFromT(context.Background(), t)
			defer cancel()

			srv, err := provisioner.New(nil, tc.client)
			if err != nil {
				t.Fatalf("failed to create provisioner server: %v", err)
			}

			actual, err := srv.DriverCreateBucket(ctx, tc.request)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(tc.expectedResponse, actual) {
				t.Errorf("expected response to be deeply equal to: %v, but got: %v",
					tc.expectedResponse,
					actual)
			}
		})
	}
}

func TestDriverDeleteBucket(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		client        linodeclient.LinodeClient
		request       *cosi.DriverDeleteBucketRequest
		expectedError error
	}{} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testutils.ContextFromT(context.Background(), t)
			defer cancel()

			srv, err := provisioner.New(nil, tc.client)
			if err != nil {
				t.Fatalf("failed to create provisioner server: %v", err)
			}

			_, err = srv.DriverDeleteBucket(ctx, tc.request)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}

func TestDriverGrantBucketAccess(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName         string
		client           linodeclient.LinodeClient
		request          *cosi.DriverGrantBucketAccessRequest
		expectedResponse *cosi.DriverGrantBucketAccessResponse
		expectedError    error
	}{} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testutils.ContextFromT(context.Background(), t)
			defer cancel()

			srv, err := provisioner.New(nil, tc.client)
			if err != nil {
				t.Fatalf("failed to create provisioner server: %v", err)
			}

			actual, err := srv.DriverGrantBucketAccess(ctx, tc.request)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}

			if !reflect.DeepEqual(tc.expectedResponse, actual) {
				t.Errorf("expected response to be deeply equal to: %v, but got: %v",
					tc.expectedResponse,
					actual)
			}
		})
	}
}

func TestDriverRevokeBucketAccess(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		request       *cosi.DriverRevokeBucketAccessRequest
		client        linodeclient.LinodeClient
		expectedError error
	}{} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testutils.ContextFromT(context.Background(), t)
			defer cancel()

			srv, err := provisioner.New(nil, tc.client)
			if err != nil {
				t.Fatalf("failed to create provisioner server: %v", err)
			}

			_, err = srv.DriverRevokeBucketAccess(ctx, tc.request)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}
