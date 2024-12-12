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

package provisioner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linodego"
)

// Server implements cosi.ProvisionerServer interface.
type Server struct {
	log  *slog.Logger
	once sync.Once

	client linodeclient.Client
}

// Interface guards.
var _ cosi.ProvisionerServer = (*Server)(nil)

// New returns provisioner.Server with default values.
func New(logger *slog.Logger, client linodeclient.Client) (*Server, error) {
	srv := &Server{
		log:    logger,
		client: client,
	}

	return srv, nil
}

func (s *Server) logAttr(attr ...slog.Attr) *slog.Logger {
	s.once.Do(func() {
		if s.log == nil {
			s.log = slog.Default()
		}
	})

	return slog.New(s.log.Handler().WithAttrs(attr))
}

// DriverCreateBucket call is made to create the bucket in the backend.
//
// NOTE: this call needs to be idempotent.
//  1. If a bucket that matches both name and parameters already exists, then OK (success) must be returned.
//  2. If a bucket by same name, but different parameters is provided, then the appropriate error code ALREADY_EXISTS must be returned.
func (s *Server) DriverCreateBucket(ctx context.Context, req *cosi.DriverCreateBucketRequest) (*cosi.DriverCreateBucketResponse, error) {
	label := req.GetName()
	region := req.GetParameters()[ParamRegion]
	cors := ParamCORSValue(req.GetParameters()[ParamCORS])

	acl := linodego.ObjectStorageACL(req.GetParameters()[ParamACL])
	if acl == "" {
		acl = linodego.ACLPrivate
	}

	log := s.logAttr(
		slog.String(KeyBucketRegion, region),
		slog.String(KeyBucketLabel, label),
	).WithGroup("DriverCreateBucket")

	log.InfoContext(ctx, "Bucket creation initiated")

	if region == "" {
		log.ErrorContext(ctx, "Required parameter was not provided in the request", "error", ErrMissingRegion)

		return nil, status.Error(codes.InvalidArgument, "region was not provided")
	}

	bucket, err := s.client.GetObjectStorageBucket(ctx, region, label)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			log.ErrorContext(ctx, "Failed to check if bucket exists", "error", err)
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to check if bucket exists: %v", err))
		}

		opts := linodego.ObjectStorageBucketCreateOptions{
			Region:      region,
			Label:       label,
			ACL:         acl,
			CorsEnabled: cors.BoolP(),
		}

		log.InfoContext(ctx, "Creating bucket")

		bucket, err = s.client.CreateObjectStorageBucket(ctx, opts)
		if err != nil {
			log.ErrorContext(ctx, "Failed to create bucket", "error", err)
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create bucket: %v", err))
		}

		log.InfoContext(ctx, "Bucket created")

		return &cosi.DriverCreateBucketResponse{
			BucketId:   bucket.Region + "/" + bucket.Label,
			BucketInfo: bucketInfo(bucket.Region),
		}, status.Error(codes.OK, "bucket created")
	}

	log.DebugContext(ctx, "Bucket found, checking bucket access",
		KeyBucketCreationTimestamp, bucket.Region,
	)

	access, err := s.client.GetObjectStorageBucketAccess(ctx, region, label)
	if err != nil {
		log.ErrorContext(ctx, "Failed to check bucket access", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to check bucket access: %v", err))
	}

	if access.ACL != acl || access.CorsEnabled != cors.Bool() {
		log.ErrorContext(ctx, "Bucket with different parameters already exists",
			"existing_"+KeyBucketACL, access.ACL,
			"existing_"+KeyBucketCORS, access.CorsEnabled,
		)

		return nil, status.Error(codes.AlreadyExists, "bucket exists with different parameters")
	}

	log.InfoContext(ctx, "Bucket exists")

	return &cosi.DriverCreateBucketResponse{
		BucketId:   bucket.Region + "/" + bucket.Label,
		BucketInfo: bucketInfo(bucket.Region),
	}, status.Error(codes.OK, "bucket exists")
}

// DriverDeleteBucket call is made to delete the bucket in the backend.
//
// NOTE: this call needs to be idempotent.
// If the bucket has already been deleted, then no error should be returned.
func (s *Server) DriverDeleteBucket(ctx context.Context, req *cosi.DriverDeleteBucketRequest) (*cosi.DriverDeleteBucketResponse, error) {
	region, label := parseBucketID(req.GetBucketId())

	log := s.logAttr(
		slog.String(KeyBucketID, req.GetBucketId()),
		slog.String(KeyBucketRegion, region),
		slog.String(KeyBucketLabel, label),
	).WithGroup("DriverDeleteBucket")

	log.InfoContext(ctx, "Bucket deletion initiated")

	err := s.client.DeleteObjectStorageBucket(ctx, region, label)
	if err == nil || errors.Is(err, ErrNotFound) {
		log.InfoContext(ctx, "Bucket deleted")
		return &cosi.DriverDeleteBucketResponse{}, status.Error(codes.OK, "bucket deleted")
	}

	log.ErrorContext(ctx, "Failed to delete bucket", "error", err)

	return nil, status.Error(codes.Internal, fmt.Sprintf("failed to delete bucket: %v", err))
}

// DriverGrantBucketAccess call grants access to an account.
// The account_name in the request shall be used as a unique identifier to create credentials.
//
// NOTE: this call needs to be idempotent.
// The account_id returned in the response will be used as the unique identifier for deleting this access when calling DriverRevokeBucketAccess.
// The returned secret does not need to be the same each call to achieve idempotency.
func (s *Server) DriverGrantBucketAccess(ctx context.Context, req *cosi.DriverGrantBucketAccessRequest) (*cosi.DriverGrantBucketAccessResponse, error) {
	region, label := parseBucketID(req.GetBucketId())
	name := req.GetName()
	auth := req.GetAuthenticationType()
	perms := ParamPermissionsValue(req.GetParameters()[ParamPermissions])

	if perms == "" {
		perms = ParamPermissionsValueReadOnly
	}

	log := s.logAttr(
		slog.String(KeyBucketID, req.GetBucketId()),
		slog.String(KeyBucketRegion, region),
		slog.String(KeyBucketLabel, label),
		slog.String(KeyBucketAccessName, name),
		slog.Any(KeyBucketAccessAuth, auth),
		slog.Any(KeyBucketAccessPermissions, perms),
	).WithGroup("DriverGrantBucketAccess")

	if auth != cosi.AuthenticationType_Key {
		log.ErrorContext(ctx, "Unsupported authentication type")

		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%v: %s", ErrUnsuportedAuth, auth.String()))
	} else if perms != ParamPermissionsValueReadOnly && perms != ParamPermissionsValueReadWrite {
		log.ErrorContext(ctx, "Unknown permissions")

		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("%v: %s", ErrUnknownPermsissions, perms))
	}

	opts := linodego.ObjectStorageKeyCreateOptions{
		Label: name,
		BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
			{
				Region:      region,
				BucketName:  label,
				Permissions: string(perms),
			},
		},
	}

	log.InfoContext(ctx, "Creating object storage key")

	key, err := s.client.CreateObjectStorageKey(ctx, opts)
	if err != nil {
		log.ErrorContext(ctx, "Failed to create object storage key", "error", err)
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to create object storage key: %v", err))
	}

	log.InfoContext(ctx, "Object storage key created")

	return &cosi.DriverGrantBucketAccessResponse{
		AccountId:   fmt.Sprintf("%d", key.ID),
		Credentials: credentials(region, label, key.AccessKey, key.SecretKey),
	}, status.Error(codes.OK, "bucket access granted")
}

// DriverRevokeBucketAccess call revokes all access to a particular bucket from a principal.
//
// NOTE: this call needs to be idempotent.
func (s *Server) DriverRevokeBucketAccess(ctx context.Context, req *cosi.DriverRevokeBucketAccessRequest) (*cosi.DriverRevokeBucketAccessResponse, error) {
	region, label := parseBucketID(req.GetBucketId())
	id, err := strconv.Atoi(req.GetAccountId())

	log := s.logAttr(
		slog.String(KeyBucketID, req.GetBucketId()),
		slog.String(KeyBucketAccessIDRaw, req.GetBucketId()),
		slog.String(KeyBucketRegion, region),
		slog.String(KeyBucketLabel, label),
		slog.Int(KeyBucketAccessID, id),
	).WithGroup("DriverRevokeBucketAccess")

	if err != nil {
		log.ErrorContext(ctx, "Invalid account id", "error", err)
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("account id is invalid: %v", err))
	}

	err = s.client.DeleteObjectStorageKey(ctx, id)
	if err == nil || errors.Is(err, ErrNotFound) {
		log.InfoContext(ctx, "Key deleted")
		return &cosi.DriverRevokeBucketAccessResponse{}, status.Error(codes.OK, "key deleted")
	}

	log.ErrorContext(ctx, "Failed to delete key", "error", err)

	return nil, status.Error(codes.Internal, fmt.Sprintf("failed to delete key: %v", err))
}
