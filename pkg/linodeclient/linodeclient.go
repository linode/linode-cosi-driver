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
	"log/slog"
	"strings"

	"github.com/google/uuid"
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

	ListObjectStorageEndpoints(context.Context, *linodego.ListOptions) ([]linodego.ObjectStorageEndpoint, error)
}

// NewLinodeClient takes userAgent prefix after initial validation
// returns new linodego Client. The client uses linodego built-in http client
// which supports setting root CA cert.
func NewLinodeClient(ua string) (*linodego.Client, error) {
	linodeClient, err := linodego.NewClientFromEnv(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create linode client from env: %w", err)
	}

	linodeClient.SetUserAgent(ua)

	return linodeClient, nil
}

func NewEphemeralS3Credentials(
	ctx context.Context,
	slog *slog.Logger,
	client *linodego.Client,
) (*linodego.ObjectStorageKey, func(context.Context) error, error) {
	keyLabel := fmt.Sprintf("cosi-%s", uuid.New().String())
	slog.Info(fmt.Sprintf("Generating new ephemeral key: %s", keyLabel))

	clusters, err := client.ListObjectStorageEndpoints(ctx, &linodego.ListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list ObjectStorage clusters: %w", err)
	}

	var regions = make([]string, 0, len(clusters))
	for _, cluster := range clusters {
		regions = append(regions, cluster.Region)
	}

	creds, err := client.CreateObjectStorageKey(ctx, linodego.ObjectStorageKeyCreateOptions{
		Label:   keyLabel,
		Regions: regions,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create object storage key: %w. requested regions were: %s", err, strings.Join(regions, ", "))
	}

	cleanup := func(cctx context.Context) error {
		return client.DeleteObjectStorageKey(cctx, creds.ID)
	}

	return creds, cleanup, nil
}
