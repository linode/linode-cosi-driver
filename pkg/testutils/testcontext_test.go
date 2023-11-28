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

package testutils_test

import (
	"context"
	"testing"
	"time"

	"github.com/linode/linode-cosi-driver/pkg/testutils"
)

var DefaultTimeout = time.Second * 30

func TestContextFromT(t *testing.T) {
	t.Parallel()

	// smoke test
	_, _ = testutils.ContextFromT(context.Background(), t)
}

func TestContextFromTimeout(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName string
		timeout  time.Duration
	}{
		{
			testName: "smoke small timeout",
			timeout:  time.Microsecond,
		},
		{
			testName: "smoke large timeout",
			timeout:  time.Hour * 2048,
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			// smoke test
			_, _ = testutils.ContextFromTimeout(context.Background(), t, tc.timeout)
		})
	}
}

func TestContextFromDeadline(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName string
		deadline time.Time
	}{
		{
			testName: "smoke small deadline",
			deadline: time.Now().Add(time.Microsecond),
		},
		{
			testName: "smoke large deadline",
			deadline: time.Now().Add(time.Hour * 2048),
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()
			// smoke test
			_, _ = testutils.ContextFromDeadline(context.Background(), t, tc.deadline)
		})
	}
}
