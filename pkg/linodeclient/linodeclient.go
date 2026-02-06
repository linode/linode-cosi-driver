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
	log *slog.Logger,
	client *linodego.Client,
	regions []string,
) (*linodego.ObjectStorageKey, func(context.Context) error, error) {
	keyLabel := fmt.Sprintf("cosi-%s", uuid.New().String())
	log.Info("Generating new ephemeral key", "label", keyLabel)

	keyRegions := make([]string, 0, 1)
	var clusters []linodego.ObjectStorageCluster
	if len(regions) > 0 {
		var err error
		clusters, err = client.ListObjectStorageClusters(ctx, &linodego.ListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list ObjectStorage clusters: %w", err)
		}

		requested := make(map[string]struct{}, len(regions))
		for _, region := range regions {
			requested[region] = struct{}{}
		}
		for _, cluster := range clusters {
			if _, ok := requested[cluster.Region]; ok {
				keyRegions = append(keyRegions, cluster.Region)
			}
		}
		if len(keyRegions) == 0 {
			return nil, nil, fmt.Errorf("requested regions %v not found in object storage clusters", regions)
		}
	}

	opts := linodego.ObjectStorageKeyCreateOptions{
		Label:   keyLabel,
		Regions: keyRegions,
	}
	if len(regions) == 0 {
		opts.Regions = nil
	}

	creds, err := client.CreateObjectStorageKey(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create object storage key: %w", err)
	}

	cleanup := func(cctx context.Context) error {
		return client.DeleteObjectStorageKey(cctx, creds.ID)
	}

	return creds, cleanup, nil
}
