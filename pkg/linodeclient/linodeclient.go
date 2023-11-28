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
	"fmt"
	"net/url"
	"strings"

	"github.com/linode/linodego"
)

// LinodeClient defines a subset of all Linode Client methods required by COSI.
type LinodeClient interface {
	CreateObjectStorageBucket(context.Context, linodego.ObjectStorageBucketCreateOptions) (*linodego.ObjectStorageBucket, error)
	GetObjectStorageBucket(context.Context, string, string) (*linodego.ObjectStorageBucket, error)
	DeleteObjectStorageBucket(context.Context, string, string) error

	CreateObjectStorageKey(context.Context, linodego.ObjectStorageKeyCreateOptions) (*linodego.ObjectStorageKey, error)
	ListObjectStorageKeys(context.Context, *linodego.ListOptions) ([]linodego.ObjectStorageKey, error)
	GetObjectStorageKey(context.Context, int) (*linodego.ObjectStorageKey, error)
	DeleteObjectStorageKey(context.Context, int) error
}

// NewLinodeClient takes token, userAgent prefix, and API URL and after initial validation
// returns new linodego Client. The client uses linodego built-in http client
// which supports setting root CA cert.
func NewLinodeClient(token, ua, apiURL string) (*linodego.Client, error) {
	linodeClient := linodego.NewClient(nil)
	linodeClient.SetUserAgent(ua)
	linodeClient.SetToken(token)

	if apiURL != "" {
		host, version, err := getAPIURLComponents(apiURL)
		if err != nil {
			return nil, err
		}

		linodeClient.SetBaseURL(host)

		if version != "" {
			linodeClient.SetAPIVersion(version)
		}
	}

	return &linodeClient, nil
}

// getAPIURLComponents returns the API URL components (base URL, api version) given an input URL.
// This is necessary due to some recent changes with how linodego handles
// client.SetBaseURL(...) and client.SetAPIVersion(...)
func getAPIURLComponents(apiURL string) (string, string, error) {
	u, err := url.Parse(apiURL)
	if err != nil {
		return "", "", err
	}

	// in case of missing scheme force https to prevent panic in the SetBaseURL
	if u.Scheme == "" {
		u.Scheme = "https://"
	}

	version := ""
	host := fmt.Sprintf("%s://%s", u.Scheme, u.Host)

	if strings.ReplaceAll(u.Path, "/", "") == "" {
		pathSegments := strings.Split(u.Path, "/")
		// The API version will be the last path value
		version = pathSegments[len(pathSegments)-1]
	}

	return host, version, nil
}
