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

// Package stubclient provides a stub implementation of the linodeclient.LinodeClient interface.
// This is intended for testing purposes.
package stubclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
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
	objectStorageBuckets        map[string]*linodego.ObjectStorageBucket
	objectStorageBucketAccesses map[string]*linodego.ObjectStorageBucketAccess
	objectStorageKeys           map[int]*linodego.ObjectStorageKey
}

var _ linodeclient.Client = (*Client)(nil)

// New creates a new instance of the Client with optional object storage objects.
// This is a stub function.
func New(opts ...Option) *Client {
	c := &Client{
		objectStorageBuckets:        make(map[string]*linodego.ObjectStorageBucket),
		objectStorageBucketAccesses: make(map[string]*linodego.ObjectStorageBucketAccess),
		objectStorageKeys:           make(map[int]*linodego.ObjectStorageKey),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func validateACL(acl linodego.ObjectStorageACL) bool {
	switch acl {
	case linodego.ACLPrivate, linodego.ACLAuthenticatedRead, linodego.ACLPublicRead, linodego.ACLPublicReadWrite:
		return true
	default:
		return false
	}
}

func handleForcedFailure(ctx context.Context) error {
	if v := ctx.Value(ForcedFailure); v != nil {
		switch v := v.(type) {
		case error:
			return v
		default:
			return ErrUnexpectedError
		}
	}

	return nil
}

// CreateObjectStorageBucket is a stub function that stubs the behavior of CreateObjectStorageBucket call from linodego.Client.
func (c *Client) CreateObjectStorageBucket(ctx context.Context, opt linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error) {
	if err := handleForcedFailure(ctx); err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s/%s", opt.Region, opt.Label)

	bucket, ok := c.objectStorageBuckets[key]
	if ok {
		return bucket, nil
	}

	if !validateACL(opt.ACL) {
		return nil, &linodego.Error{
			Code: http.StatusBadRequest,
		}
	}

	bucket = &linodego.ObjectStorageBucket{
		Label:    opt.Label,
		Region:   opt.Region,
		Hostname: fmt.Sprintf("%s.linodeobjects.com", opt.Label),
	}
	c.objectStorageBuckets[key] = bucket

	corsEnabled := false
	if opt.CorsEnabled != nil {
		corsEnabled = *opt.CorsEnabled
	}

	access := &linodego.ObjectStorageBucketAccess{
		ACL:         opt.ACL,
		CorsEnabled: corsEnabled,
	}
	c.objectStorageBucketAccesses[key] = access

	return bucket, nil
}

// GetObjectStorageBucket is a stub function that stubs the behavior of GetObjectStorageBucket call from linodego.Client.
func (c *Client) GetObjectStorageBucket(ctx context.Context, region, label string) (*linodego.ObjectStorageBucket, error) {
	if err := handleForcedFailure(ctx); err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s/%s", region, label)

	obj, ok := c.objectStorageBuckets[key]
	if ok {
		return obj, nil
	}

	return nil, &linodego.Error{
		Code: http.StatusNotFound,
	}
}

// DeleteObjectStorageBucket is a stub function that stubs the behavior of DeleteObjectStorageBucket call from linodego.Client.
func (c *Client) DeleteObjectStorageBucket(ctx context.Context, region, label string) error {
	if err := handleForcedFailure(ctx); err != nil {
		return err
	}

	key := fmt.Sprintf("%s/%s", region, label)

	bucket, ok := c.objectStorageBuckets[key]

	switch {
	case ok && (bucket.Objects == 0 && bucket.Size == 0):
		delete(c.objectStorageBuckets, key)
		return nil
	case ok && (bucket.Objects != 0 || bucket.Size > 0):
		return &linodego.Error{
			Code: http.StatusBadRequest,
		}
	default:
		return &linodego.Error{
			Code: http.StatusNotFound,
		}
	}
}

// GetObjectStorageBucketAccess is a stub function that stubs the behavior of GetObjectStorageBucketAccess call from linodego.Client.
func (c *Client) GetObjectStorageBucketAccess(ctx context.Context, region, label string) (*linodego.ObjectStorageBucketAccess, error) {
	if err := handleForcedFailure(ctx); err != nil {
		return nil, err
	}

	key := fmt.Sprintf("%s/%s", region, label)

	obj, ok := c.objectStorageBucketAccesses[key]
	if ok {
		return obj, nil
	}

	return nil, &linodego.Error{
		Code: http.StatusNotFound,
	}
}

// UpdateObjectStorageBucketAccess is a stub function that stubs the behavior of UpdateObjectStorageBucketAccess call from linodego.Client.
func (c *Client) UpdateObjectStorageBucketAccess(ctx context.Context, region, label string, opt linodego.ObjectStorageBucketUpdateAccessOptions) error {
	if err := handleForcedFailure(ctx); err != nil {
		return err
	}

	key := fmt.Sprintf("%s/%s", region, label)

	access, ok := c.objectStorageBucketAccesses[key]
	if !ok {
		return &linodego.Error{
			Code: http.StatusNotFound,
		}
	}

	corsEnabled := access.CorsEnabled
	if opt.CorsEnabled != nil {
		corsEnabled = *opt.CorsEnabled
	}

	if !validateACL(opt.ACL) {
		return &linodego.Error{
			Code: http.StatusBadRequest,
		}
	}

	access = &linodego.ObjectStorageBucketAccess{
		ACL:         opt.ACL,
		CorsEnabled: corsEnabled,
	}
	c.objectStorageBucketAccesses[key] = access

	return nil
}

// CreateObjectStorageKey is a stub function that stubs the behavior of CreateObjectStorageKey call from linodego.Client.
func (c *Client) CreateObjectStorageKey(ctx context.Context, opt linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error) {
	if err := handleForcedFailure(ctx); err != nil {
		return nil, err
	}

	limited := false
	if opt.BucketAccess != nil && len(*opt.BucketAccess) != 0 {
		limited = true
	}

	obj := &linodego.ObjectStorageKey{
		Label:        opt.Label,
		AccessKey:    TestAccessKey,
		SecretKey:    TestSecretKey,
		BucketAccess: opt.BucketAccess,
		Limited:      limited,
	}

	id := len(c.objectStorageKeys)
	c.objectStorageKeys[id] = obj

	return obj, nil
}

// ListObjectStorageKeys is a stub function that stubs the behavior of ListObjectStorageKeys call from linodego.Client.
func (c *Client) ListObjectStorageKeys(ctx context.Context, opt *linodego.ListOptions) ([]linodego.ObjectStorageKey, error) {
	if err := handleForcedFailure(ctx); err != nil {
		return nil, err
	}

	list := []linodego.ObjectStorageKey{}

	for _, obj := range c.objectStorageKeys {
		list = append(list, *obj)
	}

	if opt != nil && opt.PageOptions != nil {
		if opt.PageSize < 0 {
			opt.PageSize = 100
		}

		if opt.Page <= 0 {
			opt.Page = 1
		}

		startIndex := (opt.Page - 1) * opt.PageSize

		endIndex := startIndex + opt.PageSize

		// check for out-of-bounds
		if startIndex >= len(list) {
			return []linodego.ObjectStorageKey{}, nil
		}

		// adjust endIndex if it exceeds the length of the slice
		if endIndex > len(list) {
			endIndex = len(list)
		}

		// return the specified page
		return list[startIndex:endIndex], nil
	}

	return list, nil
}

// GetObjectStorageKey is a stub function that stubs the behavior of GetObjectStorageKey call from linodego.Client.
func (c *Client) GetObjectStorageKey(ctx context.Context, id int) (*linodego.ObjectStorageKey, error) {
	if err := handleForcedFailure(ctx); err != nil {
		return nil, err
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
	if err := handleForcedFailure(ctx); err != nil {
		return err
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
