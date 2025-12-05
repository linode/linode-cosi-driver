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

package stubclient

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"

	"github.com/linode/linodego"
	"github.com/minio/minio-go/v7"

	"github.com/linode/linode-cosi-driver/pkg/s3"
)

const (
	// ForcedFailure is a context key used to simulate forced failures in certain methods.
	ForcedFailure = "X_Forced_Failure"
)

type BucketTracker interface {
	GetObjectStorageBucket(context.Context, string, string) (*linodego.ObjectStorageBucket, error)
}

func SetBucketTracker(c s3.Client, tracker BucketTracker) {
	if c, ok := c.(*Client); ok {
		c.tracker = tracker
	}
}

// ErrUnexpectedError represents an unexpected error during the stub implementation.
var ErrUnexpectedError = errors.New("unexpected error")

type Client struct {
	tracker  BucketTracker
	policies map[string]string
}

var _ s3.Client = (*Client)(nil)

// New creates a new instance of the Client with optional object storage objects.
// This is a stub function.
func New(opts ...Option) *Client {
	client := &Client{
		policies: make(map[string]string),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func handleForcedFailure(ctx context.Context) error {
	if val := ctx.Value(ForcedFailure); val != nil {
		switch vType := val.(type) {
		case error:
			return vType
		default:
			return ErrUnexpectedError
		}
	}

	return nil
}

// Prune is a stub function that stubs the behavior of Prune call from s3.Client.
func (c *Client) Prune(ctx context.Context, _ string, _ string) error {
	if err := handleForcedFailure(ctx); err != nil {
		return err
	}

	return nil
}

// PolicyDocument represents the structure of an AWS S3 bucket policy.
type PolicyDocument struct {
	ID        string      `json:"Id"`
	Version   string      `json:"Version"`
	Statement []Statement `json:"Statement"`
}

// Statement represents an individual statement in the policy.
type Statement struct {
	SID       string `json:"Sid"`
	Effect    string `json:"Effect"`
	Action    any    `json:"Action"`    // Can be a string or a slice
	Resource  any    `json:"Resource"`  // Can be a string or a slice
	Principal any    `json:"Principal"` // Can be a string, object, or slice
}

// validatePolicy checks if the given policy is a valid AWS S3 bucket policy.
func validatePolicy(policy string) bool {
	if policy == "" {
		return true
	}

	var doc PolicyDocument
	if err := json.Unmarshal([]byte(policy), &doc); err != nil {
		return false // Invalid JSON format
	}

	// Check required fields
	if doc.Version == "" || len(doc.Statement) == 0 {
		return false
	}

	for _, stmt := range doc.Statement {
		if stmt.Effect == "" || stmt.Action == nil || stmt.Resource == nil {
			return false
		}

		// Ensure Action, Resource, and Principal are properly structured
		if !isValidStringOrSlice(stmt.Action) || !isValidStringOrSlice(stmt.Resource) || !isValidPrincipal(stmt.Principal) {
			return false
		}
	}

	return true
}

// isValidStringOrSlice ensures the field is either a string or a slice of strings.
func isValidStringOrSlice(value any) bool {
	switch vType := value.(type) {
	case string:
		return vType != ""

	case []any:
		if len(vType) == 0 {
			return false
		}

		for _, item := range vType {
			if _, ok := item.(string); !ok {
				return false
			}
		}

		return true

	default:
		return false
	}
}

// isValidPrincipal ensures the Principal field is a valid type.
func isValidPrincipal(value any) bool {
	switch vType := value.(type) {
	case string:
		return vType != ""

	case map[string]any:
		if len(vType) == 0 {
			return false
		}

		for _, item := range vType {
			if !isValidStringOrSlice(item) {
				return false
			}
		}

		return true

	case []any:
		return isValidStringOrSlice(vType)

	default:
		return false
	}
}

func (c *Client) SetBucketPolicy(ctx context.Context, region, bucket, policy string) error {
	if err := handleForcedFailure(ctx); err != nil {
		return err
	}

	id := fmt.Sprintf("%s/%s", region, bucket)

	if c.tracker != nil {
		if _, err := c.tracker.GetObjectStorageBucket(ctx, region, bucket); err == nil {
			c.policies[id] = ""
		}
	}

	_, ok := c.policies[id]
	if !ok {
		return minio.ErrorResponse{
			XMLName:    xml.Name{Local: "Error"},
			Code:       "NoSuchBucket",
			BucketName: bucket,
			StatusCode: http.StatusNotFound,
		}
	}

	if ok := validatePolicy(policy); !ok {
		return minio.ErrorResponse{
			XMLName:    xml.Name{Local: "Error"},
			Code:       "InvalidArgument",
			BucketName: bucket,
			StatusCode: http.StatusBadRequest,
		}
	}

	c.policies[id] = policy

	return nil
}

func (c *Client) GetBucketPolicy(ctx context.Context, region, bucket string) (string, error) {
	if err := handleForcedFailure(ctx); err != nil {
		return "", err
	}

	id := fmt.Sprintf("%s/%s", region, bucket)

	policy, ok := c.policies[id]
	if !ok {
		return "", minio.ErrorResponse{
			XMLName:    xml.Name{Local: "Error"},
			Code:       "NoSuchBucket",
			BucketName: bucket,
			StatusCode: http.StatusNotFound,
		}
	}

	return policy, nil
}
