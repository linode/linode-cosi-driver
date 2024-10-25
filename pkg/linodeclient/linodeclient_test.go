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

//go:build !integration
// +build !integration

package linodeclient

import (
	"errors"
	"testing"
)

func TestNewLinodeClient(t *testing.T) {
	t.Parallel()

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
			testName:  "with user agent",
			userAgent: "test_UA",
		},
		{
			testName: "with token",
			token:    "test_TOKEN",
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

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			_, err := NewLinodeClient(tc.token, tc.userAgent, tc.url, tc.version)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}
