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
	"context"
	"net/url"
	"regexp"

	"github.com/linode/linodego"
)

// Client defines a subset of all Linode Client methods required by COSI.
type Client interface {
	CreateObjectStorageBucket(context.Context, linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error)
	GetObjectStorageBucket(context.Context, string, string) (*linodego.ObjectStorageBucket, error)
	DeleteObjectStorageBucket(context.Context, string, string) error

	GetObjectStorageBucketAccess(context.Context, string, string) (*linodego.ObjectStorageBucketAccess, error)
	UpdateObjectStorageBucketAccess(context.Context, string, string, linodego.ObjectStorageBucketUpdateAccessOptions) error

	CreateObjectStorageKey(context.Context, linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error)
	ListObjectStorageKeys(context.Context, *linodego.ListOptions) ([]linodego.ObjectStorageKey, error)
	GetObjectStorageKey(context.Context, int) (*linodego.ObjectStorageKey, error)
	DeleteObjectStorageKey(context.Context, int) error
}

// NewLinodeClient takes token, userAgent prefix, and API URL and after initial validation
// returns new linodego Client. The client uses linodego built-in http client
// which supports setting root CA cert.
func NewLinodeClient(token, ua, apiURL, apiVersion string) (*linodego.Client, error) {
	linodeClient := linodego.NewClient(nil)
	linodeClient.SetUserAgent(ua)
	linodeClient.SetToken(token)

	// Validate apiURL
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}

	validatedURL := &url.URL{
		Host:   parsedURL.Host,
		Scheme: parsedURL.Scheme,
	}

	linodeClient.SetBaseURL(validatedURL.String())

	version := ""
	matches := regexp.MustCompile(`/v\d+`).FindAllString(parsedURL.Path, -1)

	if len(matches) > 0 {
		version = matches[len(matches)-1]
	}

	// If version segment is present, and apiVersion is present, use the apiVersion in linodeClient.SetAPIVersion(version)
	if apiVersion != "" {
		version = apiVersion
	}

	linodeClient.SetAPIVersion(version)

	return &linodeClient, nil
}
