// Copyright 2023 Linode, LLC
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
	"testing"

	cosi "sigs.k8s.io/container-object-storage-interface-spec"
)

func TestDriverCreateBucket(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		request       *cosi.DriverCreateBucketRequest
		expectedError error
	}{} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
		})
	}
}

func TestDriverDeleteBucket(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		request       *cosi.DriverDeleteBucketRequest
		expectedError error
	}{} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
		})
	}
}

func TestDriverGrantBucketAccess(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		request       *cosi.DriverGrantBucketAccessRequest
		expectedError error
	}{} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
		})
	}
}

func TestDriverRevokeBucketAccess(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		request       *cosi.DriverRevokeBucketAccessRequest
		expectedError error
	}{} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
		})
	}
}
