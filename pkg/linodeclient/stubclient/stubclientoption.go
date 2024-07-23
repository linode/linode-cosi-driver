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

package stubclient

import (
	"fmt"

	"github.com/linode/linodego"
)

// Option represents a function that configures a Client.
type Option func(c *Client)

// WithBucket is an option to configure the stub client with an Object Storage bucket.
func WithBucket(bucket *linodego.ObjectStorageBucket) Option {
	return func(c *Client) {
		id := fmt.Sprintf("%s/%s", bucket.Region, bucket.Label)
		c.objectStorageBuckets[id] = bucket
	}
}

// WithKey is an option to configure the stub client with an Object Storage key.
func WithKey(key *linodego.ObjectStorageKey) Option {
	return func(c *Client) {
		id := key.ID
		c.objectStorageKeys[id] = key
	}
}

// WithBucketAccess is an option to configure the stub client with Object Storage bucket access.
func WithBucketAccess(bucketAccess *linodego.ObjectStorageBucketAccess, region, label string) Option {
	return func(c *Client) {
		id := fmt.Sprintf("%s/%s", region, label)
		c.objectStorageBucketAccesses[id] = bucketAccess
	}
}
