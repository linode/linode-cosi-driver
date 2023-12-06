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

package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/linode/linode-cosi-driver/pkg/testutils"
)

func TestRealMain(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string // required
		cosi          string // required
		token         string
		url           string
		version       string
		expectedError error
	}{
		{
			testName: "simple",
			cosi:     "cosi.sock",
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := testutils.ContextFromTimeout(context.Background(), t, time.Second)
			defer cancel()

			tmp := testutils.MustMkdirTemp()
			defer os.RemoveAll(tmp)

			err := realMain(ctx, "unix://"+tmp+tc.cosi, tc.token, tc.url, tc.version)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}
