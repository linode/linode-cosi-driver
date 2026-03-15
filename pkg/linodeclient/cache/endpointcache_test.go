// Copyright 2025 Akamai Technologies, Inc.
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

package cache

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/linode/linodego"
	"go.uber.org/mock/gomock"

	"github.com/linode/linode-cosi-driver/testing/mock"
)

var discardLog = slog.New(slog.DiscardHandler)

func ptr[T any](val T) *T {
	return &val
}

func TestCache(t *testing.T) {
	t.Parallel()

	testRegion1 := linodego.ObjectStorageEndpoint{
		Region:     "us-test",
		S3Endpoint: ptr("us-test-1.linodeobjects.com"),
	}
	testRegion2 := linodego.ObjectStorageEndpoint{
		Region:     "de-test",
		S3Endpoint: ptr("de-test-1.linodeobjects.com"),
	}
	testRegion3 := linodego.ObjectStorageEndpoint{
		Region:     "pl-test",
		S3Endpoint: ptr("pl-test-1.linodeobjects.com"),
	}

	ctrl := gomock.NewController(t)
	mockClient := mock.NewMockLinodeClient(ctrl)
	mockClient.EXPECT().
		ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
		Return([]linodego.ObjectStorageEndpoint{testRegion1, testRegion2, testRegion3}, nil)

	cache := New(
		discardLog,
		mockClient,
		DefaultTTL,
	)

	err := cache.Refresh(t.Context())
	if err != nil {
		t.Errorf("cache refresh failed: %v", err)
	}

	s3Endpoint, ok := cache.Get("us-test")
	if !ok || s3Endpoint != *testRegion1.S3Endpoint {
		t.Errorf("expected %s, got %s", *testRegion1.S3Endpoint, s3Endpoint)
	}

	s3Endpoint, ok = cache.Get("de-test")
	if !ok || s3Endpoint != *testRegion2.S3Endpoint {
		t.Errorf("expected %s, got %s", *testRegion2.S3Endpoint, s3Endpoint)
	}

	s3Endpoint, ok = cache.Get("pl-test")
	if !ok || s3Endpoint != *testRegion3.S3Endpoint {
		t.Errorf("expected %s, got %s", *testRegion3.S3Endpoint, s3Endpoint)
	}

	s3Endpoint, ok = cache.Get("jp-test")
	if ok || s3Endpoint != "" {
		t.Errorf("expected empty result for unknown region, got %s", s3Endpoint)
	}
}

func TestCacheStart(t *testing.T) {
	t.Parallel()

	testRegion := linodego.ObjectStorageEndpoint{
		Region:     "us-test",
		S3Endpoint: ptr("us-test-1.linodeobjects.com"),
	}
	testTTL := time.Second

	ctrl := gomock.NewController(t)
	mockClient := mock.NewMockLinodeClient(ctrl)
	mockClient.EXPECT().
		ListObjectStorageEndpoints(gomock.Any(), gomock.Any()).
		Return([]linodego.ObjectStorageEndpoint{testRegion}, nil).
		AnyTimes()

	cache := New(
		discardLog,
		mockClient,
		testTTL,
	)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)

	go func() {
		done <- cache.Start(ctx)
	}()

	// Allow some time for refresh to happen
	time.Sleep(testTTL)

	// Verify cache has been populated
	s3Endpoint, ok := cache.Get("us-test")
	if !ok || s3Endpoint != *testRegion.S3Endpoint {
		t.Errorf("expected %s, got %s", *testRegion.S3Endpoint, s3Endpoint)
	}

	// Cancel the context to stop the cache
	cancel()

	// Ensure the function returns after canceling
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(testTTL):
		t.Fatal("cache did not stop within expected time")
	}
}
