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

package linodeclient_test

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/testing/mock"
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
			_, err := linodeclient.NewLinodeClient(tc.userAgent)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}

func TestNewEphemeralS3Credentials(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	client := mock.NewMockLinodeClient(ctrl)

	client.EXPECT().
		CreateObjectStorageKey(gomock.Any(), gomock.Any()).
		Return(&linodego.ObjectStorageKey{
			ID:    10,
			Label: "test-key",
			Regions: []linodego.ObjectStorageKeyRegion{{
				ID: "us-test",
			}}}, nil).
		Times(1)

	client.EXPECT().
		DeleteObjectStorageKey(gomock.Any(), 10).
		Return(nil).
		Times(1)
	creds, cleanup, err := linodeclient.NewEphemeralS3Credentials(
		t.Context(),
		slog.New(slog.DiscardHandler),
		client,
		"us-test",
	)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if creds.Limited {
		t.Fatal("expected ephemeral policy credentials to be region-scoped, got bucket-scoped key")
	}

	if creds.BucketAccess != nil {
		t.Fatalf("expected no bucket access restrictions, got: %+v", *creds.BucketAccess)
	}

	regionIDs := make([]string, 0, len(creds.Regions))
	for _, region := range creds.Regions {
		regionIDs = append(regionIDs, region.ID)
	}

	if len(regionIDs) != 1 || !contains(regionIDs, "us-test") {
		t.Fatalf("expected credentials for only us-test, got: %+v", regionIDs)
	}

	if err := cleanup(t.Context()); err != nil {
		t.Fatalf("expected cleanup to succeed, got: %v", err)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}

	return false
}
