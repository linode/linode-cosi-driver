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

package linodeclient

import (
	"errors"
	"testing"
)

//nolint:paralleltest // modifies environment variables
func TestNewLinodeClient(t *testing.T) {
	for _, tc := range []struct {
		testName      string // required
		token         string
		url           string
		version       string
		userAgent     string
		expectedError error
	}{
		{
			testName: "simple",
		},
		{
			testName: "with URL",
			url:      "https://example.com",
		},
		{
			testName: "with URL with version",
			url:      "https://example.com/v4",
		},
		{
			testName: "with URL and API version",
			url:      "https://example.com",
			version:  "v4",
		},
		{
			testName: "with URL with version and API version",
			url:      "https://example.com/v4",
			version:  "v4",
		},
		{
			testName: "with URL without scheme",
			url:      "example.com",
		},
		{
			testName: "with URL without scheme with version",
			url:      "example.com/v4",
		},
	} {
		tc := tc
		t.Setenv("LINODE_TOKEN", "TEST_TOKEN")
		t.Setenv("LINODE_URL", tc.url)
		t.Setenv("LINODE_API_VERSION", tc.version)
		t.Run(tc.testName, func(t *testing.T) {
			_, err := NewLinodeClient(tc.userAgent)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}
