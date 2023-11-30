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

// Package stubclient provides a stub implementation of the linodeclient.LinodeClient interface.
// This is intended for testing purposes.
package stubclient

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net/http"

	"github.com/linode/linodego"
)

const (
	// ForcedFailure is a context key used to simulate forced failures in certain methods.
	ForcedFailure = "X_Forced_Failure"

	TestAccessKey = "TEST_ACCESS_KEY"
	TestSecretKey = "TEST_SECRET_KEY"
)

// ErrUnexpectedError represents an unexpected error during the stub implementation.
var ErrUnexpectedError = errors.New("unexpected error")

// Client is a stub implementation of the linodeclient.LinodeClient interface.
// It provides placeholder methods for object storage operations.
type Client struct {
	objectStorageBuckets map[string]*linodego.ObjectStorageBucket
	objectStorageKeys    map[int]*linodego.ObjectStorageKey
}

// New creates a new instance of the Client with optional object storage objects.
// This is a stub function.
func New(objs ...interface{}) *Client {
	c := &Client{
		objectStorageBuckets: make(map[string]*linodego.ObjectStorageBucket),
		objectStorageKeys:    make(map[int]*linodego.ObjectStorageKey),
	}

	for _, obj := range objs {
		switch obj := obj.(type) {
		case *linodego.ObjectStorageBucket:
			key := fmt.Sprintf("%s/%s", obj.Cluster, obj.Label)
			c.objectStorageBuckets[key] = obj

		case *linodego.ObjectStorageKey:
			key := obj.ID
			c.objectStorageKeys[key] = obj

		default:
			panic(fmt.Sprintf("unrecognized type: %T", obj))
		}
	}

	return c
}

// CreateObjectStorageBucket is a stub function that stubs the behavior of CreateObjectStorageBucket call from linodego.Client.
func (c *Client) CreateObjectStorageBucket(ctx context.Context, opt linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error) {
	if v := ctx.Value(ForcedFailure); v != nil {
		switch v := v.(type) {
		case error:
			return nil, v
		default:
			return nil, ErrUnexpectedError
		}
	}

	key := fmt.Sprintf("%s/%s", opt.Cluster, opt.Label)

	obj, ok := c.objectStorageBuckets[key]
	if ok {
		return obj, nil
	}

	obj = &linodego.ObjectStorageBucket{
		Label:   opt.Label,
		Cluster: opt.Cluster,
	}

	// stub discards ACL and cors settings

	c.objectStorageBuckets[key] = obj

	return obj, nil
}

// GetObjectStorageBucket is a stub function that stubs the behavior of GetObjectStorageBucket call from linodego.Client.
func (c *Client) GetObjectStorageBucket(ctx context.Context, clusterID, label string) (*linodego.ObjectStorageBucket, error) {
	if v := ctx.Value(ForcedFailure); v != nil {
		switch v := v.(type) {
		case error:
			return nil, v
		default:
			return nil, ErrUnexpectedError
		}
	}

	key := fmt.Sprintf("%s/%s", clusterID, label)

	obj, ok := c.objectStorageBuckets[key]
	if ok {
		return obj, nil
	}

	return nil, &linodego.Error{
		Code: http.StatusNotFound,
	}
}

// DeleteObjectStorageBucket is a stub function that stubs the behavior of DeleteObjectStorageBucket call from linodego.Client.
func (c *Client) DeleteObjectStorageBucket(ctx context.Context, clusterID, label string) error {
	if v := ctx.Value(ForcedFailure); v != nil {
		switch v := v.(type) {
		case error:
			return v
		default:
			return ErrUnexpectedError
		}
	}

	key := fmt.Sprintf("%s/%s", clusterID, label)

	bucket, ok := c.objectStorageBuckets[key]
	if ok && bucket.Objects == 0 && bucket.Size == 0 {
		delete(c.objectStorageBuckets, key)
		return nil
	} else if ok && (bucket.Objects != 0 || bucket.Size > 0) {
		return &linodego.Error{
			Code: http.StatusBadRequest,
		}
	}

	return &linodego.Error{
		Code: http.StatusNotFound,
	}
}

// CreateObjectStorageKey is a stub function that stubs the behavior of CreateObjectStorageKey call from linodego.Client.
func (c *Client) CreateObjectStorageKey(ctx context.Context, opt linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error) {
	if v := ctx.Value(ForcedFailure); v != nil {
		switch v := v.(type) {
		case error:
			return nil, v
		default:
			return nil, ErrUnexpectedError
		}
	}

	limited := false
	if opt.BucketAccess != nil && len(*opt.BucketAccess) == 0 {
		limited = true
	}

	obj := &linodego.ObjectStorageKey{
		Label:        opt.Label,
		AccessKey:    TestAccessKey,
		SecretKey:    TestSecretKey,
		BucketAccess: opt.BucketAccess,
		Limited:      limited,
	}

	for {
		rid := rand.Int() // #nosec G404
		if _, ok := c.objectStorageKeys[rid]; !ok {
			obj.ID = rid
			c.objectStorageKeys[rid] = obj

			break
		}
	}

	return obj, nil
}

// ListObjectStorageKeys is a stub function that stubs the behavior of ListObjectStorageKeys call from linodego.Client.
func (c *Client) ListObjectStorageKeys(ctx context.Context, opt *linodego.ListOptions) ([]linodego.ObjectStorageKey, error) {
	if v := ctx.Value(ForcedFailure); v != nil {
		switch v := v.(type) {
		case error:
			return nil, v
		default:
			return nil, ErrUnexpectedError
		}
	}

	var list []linodego.ObjectStorageKey

	for _, obj := range c.objectStorageKeys {
		list = append(list, *obj)
	}

	startIndex := (opt.Page - 1) * opt.PageSize
	endIndex := startIndex + opt.PageSize

	if endIndex <= 0 {
		endIndex = len(list) - 1
	}

	// check for out-of-bounds
	if startIndex >= len(list) {
		return nil, nil
	}

	// adjust endIndex if it exceeds the length of the slice
	if endIndex > len(list) {
		endIndex = len(list)
	}

	// fail if start index is larger than end index
	if startIndex > endIndex {
		return nil, nil
	}

	// return the specified page
	return list[startIndex:endIndex], nil
}

// GetObjectStorageKey is a stub function that stubs the behavior of GetObjectStorageKey call from linodego.Client.
func (c *Client) GetObjectStorageKey(ctx context.Context, id int) (*linodego.ObjectStorageKey, error) {
	if v := ctx.Value(ForcedFailure); v != nil {
		switch v := v.(type) {
		case error:
			return nil, v
		default:
			return nil, ErrUnexpectedError
		}
	}

	obj, ok := c.objectStorageKeys[id]
	if ok {
		return obj, nil
	}

	return nil, &linodego.Error{
		Code: http.StatusNotFound,
	}
}

// DeleteObjectStorageKey is a stub function that stubs the behavior of DeleteObjectStorageKey call from linodego.Client.
func (c *Client) DeleteObjectStorageKey(ctx context.Context, id int) error {
	if v := ctx.Value(ForcedFailure); v != nil {
		switch v := v.(type) {
		case error:
			return v
		default:
			return ErrUnexpectedError
		}
	}

	_, ok := c.objectStorageKeys[id]
	if ok {
		delete(c.objectStorageKeys, id)
		return nil
	}

	return &linodego.Error{
		Code: http.StatusNotFound,
	}
}
