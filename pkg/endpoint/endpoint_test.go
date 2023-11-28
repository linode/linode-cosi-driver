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

package endpoint

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/linode/linode-cosi-driver/pkg/testutils"
)

func TestEndpointListener(t *testing.T) {
	t.Parallel()

	ctx, cancel := testutils.ContextFromT(context.Background(), t)
	defer cancel()

	for _, tc := range []struct {
		name          string
		endpoint      *Endpoint
		expectedError error
	}{
		{
			name:          "valid UNIX socket",
			endpoint:      &Endpoint{address: testutils.MustMkUnixTemp("cosi.sock")},
			expectedError: nil,
		},
		{
			name:          "valid TCP socket",
			endpoint:      &Endpoint{address: testutils.Must(url.Parse("tcp://:0"))},
			expectedError: nil,
		},
		{
			name:          "invalid endpoint",
			endpoint:      &Endpoint{address: nil},
			expectedError: ErrEmptyAddress,
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.endpoint == nil {
				t.Fatal("endpoint is required")
			}
			defer tc.endpoint.Close() // no matter what, I need it to be called, but as best effort call (not checking err)

			_, err := tc.endpoint.Listener(ctx)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}
