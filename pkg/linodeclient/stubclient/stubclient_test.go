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

package stubclient_test

import (
	"testing"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient/stubclient"
	"github.com/linode/linode-cosi-driver/pkg/testutils"
	"github.com/linode/linodego"
)

func TestNew(t *testing.T) {
	t.Parallel()

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			testName string // required
			input    []any
		}{
			{
				testName: "nil input",
			},
			{
				testName: "buckets input",
				input: []any{
					&linodego.ObjectStorageBucket{
						Label:    "test",
						Cluster:  "test",
						Hostname: "test",
					},
				},
			},
			{
				testName: "keys input",
				input: []any{
					&linodego.ObjectStorageKey{
						ID:    1,
						Label: "test",
					},
				},
			},
			{
				testName: "mixed input",
				input: []any{
					&linodego.ObjectStorageBucket{
						Label:    "test",
						Cluster:  "test",
						Hostname: "test",
					},
					&linodego.ObjectStorageKey{
						ID:    1,
						Label: "test",
					},
				},
			},
		} {
			tc := tc

			t.Run(tc.testName, func(t *testing.T) {
				t.Parallel()

				testutils.AssertNotPanics(t, func() {
					_ = stubclient.New(tc.input...)
				})
			})
		}
	})

	t.Run("panics", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			testName string // required
			input    []any
		}{
			{
				testName: "panic on ObjectStorageACL",
				input: []any{
					linodego.ObjectStorageACL(""),
				},
			},
			{
				testName: "panic on ObjectStorageBucketAccess",
				input: []any{
					&linodego.ObjectStorageBucketAccess{},
				},
			},
			{
				testName: "panic on ObjectStorageBucketCert",
				input: []any{
					&linodego.ObjectStorageBucketCert{},
				},
			},
			{
				testName: "panic on ObjectStorageKeyBucketAccess",
				input: []any{
					&linodego.ObjectStorageKeyBucketAccess{},
				},
			},
		} {
			tc := tc

			t.Run(tc.testName, func(t *testing.T) {
				t.Parallel()

				testutils.AssertPanics(t, func() {
					_ = stubclient.New(tc.input...)
				})
			})
		}
	})
}
