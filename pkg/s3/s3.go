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

package s3

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient/cache"
)

type Client interface {
	Prune(ctx context.Context, region, bucket string) error
	SetBucketPolicy(ctx context.Context, region, bucketName, policy string) error
	GetBucketPolicy(ctx context.Context, region, bucketName string) (string, error)
}

type ClientS3 struct {
	cache       cache.Cache
	s3AccessKey string
	s3SecretKey string
	s3SSL       bool
}

var _ Client = (*ClientS3)(nil)

func New(
	cache cache.Cache,
	s3AccessKey, s3SecretKey string,
	s3SSL bool,
) *ClientS3 {
	return &ClientS3{
		cache:       cache,
		s3AccessKey: s3AccessKey,
		s3SecretKey: s3SecretKey,
		s3SSL:       s3SSL,
	}
}

func (c *ClientS3) new(region string) (*minio.Client, error) {
	endpoint, ok := c.cache.Get(region)
	if !ok || endpoint == "" {
		return nil, fmt.Errorf("failed to get endpoint for region: %s", region)
	}

	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(c.s3AccessKey, c.s3SecretKey, ""),
		Region: region,
		Secure: c.s3SSL,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to instantiate client: %w", err)
	}

	return cli, nil
}

func (c *ClientS3) Prune(ctx context.Context, region, bucket string) error {
	cli, err := c.new(region)
	if err != nil {
		return err
	}

	oiChan := cli.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true})

	errChan := cli.RemoveObjects(ctx, bucket, oiChan, minio.RemoveObjectsOptions{})

	for oerr := range errChan {
		err = errors.Join(err, oerr.Err)
	}

	return err
}

func (c *ClientS3) SetBucketPolicy(ctx context.Context, region, bucket, policy string) error {
	cli, err := c.new(region)
	if err != nil {
		return err
	}

	err = cli.SetBucketPolicy(ctx, bucket, policy)
	if err == nil {
		return nil
	}

	if res := minio.ToErrorResponse(err); res.StatusCode == http.StatusOK {
		return nil
	}

	return err
}

func (c *ClientS3) GetBucketPolicy(ctx context.Context, region, bucket string) (string, error) {
	cli, err := c.new(region)
	if err != nil {
		return "", err
	}

	return cli.GetBucketPolicy(ctx, bucket)
}

func IsNotFound(err error) bool {
	res := minio.ToErrorResponse(err)
	return res.StatusCode == http.StatusNotFound
}
