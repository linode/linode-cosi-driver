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

package cache

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
)

const (
	DefaultTTL     = time.Second * 30
	defaultTimeout = time.Second * 15
)

type Cache interface {
	Get(key string) (string, bool)
}

type EndpointCache struct {
	sync.RWMutex

	log    *slog.Logger
	ttl    time.Duration
	client linodeclient.Client
	data   map[string]string
}

func New(logger *slog.Logger, client linodeclient.Client, cacheTTL time.Duration) *EndpointCache {
	if cacheTTL == 0 || cacheTTL < DefaultTTL {
		cacheTTL = DefaultTTL
	}

	return &EndpointCache{
		log:    logger,
		ttl:    cacheTTL,
		client: client,
		data:   make(map[string]string),
	}
}

func (c *EndpointCache) Start(ctx context.Context) error {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()

	if err := c.Refresh(ctx); err != nil {
		c.log.ErrorContext(ctx, "Failed to refresh cache", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
			defer cancel()

			if err := c.Refresh(ctx); err != nil {
				c.log.ErrorContext(ctx, "Failed to refresh cache", "error", err)
			}
		}
	}
}

func (c *EndpointCache) Refresh(ctx context.Context) error {
	c.log.DebugContext(ctx, "Syncing cache")

	eps, err := c.client.ListObjectStorageEndpoints(ctx, nil)
	if err != nil {
		return fmt.Errorf("unable to list ObjectStorage endpoints: %w", err)
	}

	for _, ep := range eps {
		if ep.S3Endpoint != nil {
			c.Set(ep.Region, *ep.S3Endpoint)
		}
	}

	return nil
}

func (c *EndpointCache) Insert(iter iter.Seq2[string, string]) {
	c.Lock()
	defer c.Unlock()

	maps.Insert(c.data, iter)
}

func (c *EndpointCache) Set(key, val string) {
	c.Lock()
	defer c.Unlock()
	c.data[key] = val
}

func (c *EndpointCache) Get(key string) (string, bool) {
	c.RLock()
	defer c.RUnlock()

	val, ok := c.data[key]

	return val, ok
}
