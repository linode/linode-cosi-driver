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

package identity_test

import (
	"context"
	"errors"
	"testing"

	"github.com/linode/linode-cosi-driver/pkg/servers/identity"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"
)

func TestNew(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string
		driverName    string
		expectedError error
	}{
		{
			testName:   "valid name",
			driverName: "test.cosi",
		},
		{
			testName:      "empty name",
			expectedError: identity.ErrNameEmpty,
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			_, err := identity.New(tc.driverName)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}

func TestDriverGetInfo(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName string
		request  *cosi.DriverGetInfoRequest
	}{
		{
			testName: "non-nil request",
			request:  &cosi.DriverGetInfoRequest{},
		},
		{
			testName: "nil request",
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			idSrv, err := identity.New("test.cosi")
			if err != nil {
				t.Errorf("unexpected error in server initialization: %v", err)
				t.FailNow()
			}

			_, err = idSrv.DriverGetInfo(context.Background(), tc.request)
			if err != nil {
				t.Errorf("unexpected error in DriverGetInfo call: %v", err)
			}
		})
	}
}
