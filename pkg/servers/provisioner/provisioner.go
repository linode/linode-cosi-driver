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
	"strings"
	"sync"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/pkg/observability/tracing"
	"github.com/linode/linode-cosi-driver/pkg/version"
	"github.com/linode/linodego"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Server implements cosi.ProvisionerServer interface.
type Server struct {
	log  *slog.Logger
	once sync.Once

	client     linodeclient.Client
	kubeclient client.Reader
}

// Interface guards.
var _ cosi.ProvisionerServer = (*Server)(nil)

// New returns provisioner.Server with default values.
func New(logger *slog.Logger, client linodeclient.Client, kubeclient client.Reader) (*Server, error) {
	srv := &Server{
		log:        logger,
		client:     client,
		kubeclient: kubeclient,
	}

	return srv, srv.registerMetrics()
}

// init checks if logger was initialized and starts new span.
func (s *Server) init(ctx context.Context, caller string) (context.Context, trace.Span) {
	s.once.Do(func() {
		if s.log == nil {
			s.log = slog.Default()
		}
	})

	return tracing.Start(ctx, caller)
}

func (s *Server) logAttr(attr ...slog.Attr) *slog.Logger {
	return slog.New(s.log.Handler().WithAttrs(attr))
}

// DriverCreateBucket call is made to create the bucket in the backend.
//
// NOTE: this call needs to be idempotent.
//  1. If a bucket that matches both name and parameters already exists, then OK (success) must be returned.
//  2. If a bucket by same name, but different parameters is provided, then the appropriate error code ALREADY_EXISTS must be returned.
func (s *Server) DriverCreateBucket(ctx context.Context, req *cosi.DriverCreateBucketRequest) (*cosi.DriverCreateBucketResponse, error) {
	ctx, span := s.init(ctx, "DriverCreateBucket")
	defer span.End()

	var (
		client linodeclient.Client = s.client
		err    error
	)

	if ref, ok := req.GetParameters()[ParamSecretRef]; ok {
		client, err = s.scopedClient(ctx, ref)
		if err != nil {
			return nil, tracing.Error(span, codes.Internal, err)
		}
	}

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

	span.SetAttributes(
		attribute.String(KeyBucketRegion, region),
		attribute.String(KeyBucketLabel, label),
	)

	log.InfoContext(ctx, "bucket creation initiated")

	if region == "" {
		log.ErrorContext(ctx, "required parameter was not provided in the request", "error", ErrMissingRegion)

		return nil, tracing.Error(span, codes.InvalidArgument, ErrMissingRegion)
	}

	bucket, err := client.GetObjectStorageBucket(ctx, region, label)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			log.ErrorContext(ctx, "failed to check if bucket exists", "error", err)
			return nil, tracing.Error(span, codes.Internal, fmt.Errorf("failed to check if bucket exists: %w", err))
		}

		opts := linodego.ObjectStorageBucketCreateOptions{
			Region:      region,
			Label:       label,
			ACL:         acl,
			CorsEnabled: cors.BoolP(),
		}

		log.InfoContext(ctx, "creating bucket")

		bucket, err = client.CreateObjectStorageBucket(ctx, opts)
		if err != nil {
			log.ErrorContext(ctx, "failed to create bucket", "error", err)
			return nil, tracing.Error(span, codes.Internal, fmt.Errorf("failed to create bucket: %w", err))
		}

		log.InfoContext(ctx, "bucket created")

		return &cosi.DriverCreateBucketResponse{
			BucketId:   bucket.Region + "/" + bucket.Label,
			BucketInfo: bucketInfo(bucket.Region),
		}, tracing.Error(span, codes.OK, nil, "bucket created")
	}

	log.DebugContext(ctx, "bucket found, checking bucket access",
		KeyBucketCreationTimestamp, bucket.Region,
	)

	access, err := client.GetObjectStorageBucketAccess(ctx, region, label)
	if err != nil {
		log.ErrorContext(ctx, "failed to check bucket access", "error", err)
		return nil, tracing.Error(span, codes.Internal, fmt.Errorf("failed to check bucket access: %w", err))
	}

	if access.ACL != acl || access.CorsEnabled != cors.Bool() {
		log.ErrorContext(ctx, "bucket with different parameters already exists",
			"existing_"+KeyBucketACL, access.ACL,
			"existing_"+KeyBucketCORS, access.CorsEnabled,
		)

		return nil, tracing.Error(span, codes.AlreadyExists, ErrBucketExists)
	}

	log.InfoContext(ctx, "bucket exists")

	return &cosi.DriverCreateBucketResponse{
		BucketId:   bucket.Region + "/" + bucket.Label,
		BucketInfo: bucketInfo(bucket.Region),
	}, tracing.Error(span, codes.OK, nil, "bucket exists")
}

// DriverDeleteBucket call is made to delete the bucket in the backend.
//
// NOTE: this call needs to be idempotent.
// If the bucket has already been deleted, then no error should be returned.
func (s *Server) DriverDeleteBucket(ctx context.Context, req *cosi.DriverDeleteBucketRequest) (*cosi.DriverDeleteBucketResponse, error) {
	ctx, span := s.init(ctx, "DriverDeleteBucket")
	defer span.End()

	// TODO: requires COSI v1alpha2
	client := s.client
	// var (
	// 	client linodeclient.Client = s.client
	// 	err    error
	// )
	//
	// if ref, ok := req.GetParameters()[ParamSecretRef]; ok {
	// 	client, err = s.scopedClient(ctx, ref)
	// 	if err != nil {
	// 		return nil, tracing.Error(span, codes.Internal, err)
	// 	}
	// }

	region, label := parseBucketID(req.GetBucketId())

	log := s.logAttr(
		slog.String(KeyBucketID, req.GetBucketId()),
		slog.String(KeyBucketRegion, region),
		slog.String(KeyBucketLabel, label),
	).WithGroup("DriverDeleteBucket")

	span.SetAttributes(
		attribute.String(KeyBucketID, req.GetBucketId()),
		attribute.String(KeyBucketRegion, region),
		attribute.String(KeyBucketLabel, label),
	)

	log.InfoContext(ctx, "bucket deletion initiated")

	err := client.DeleteObjectStorageBucket(ctx, region, label)
	if err == nil || errors.Is(err, ErrNotFound) {
		log.InfoContext(ctx, "bucket deleted")
		return &cosi.DriverDeleteBucketResponse{}, tracing.Error(span, codes.OK, err, "bucket deleted")
	}

	log.ErrorContext(ctx, "failed to delete bucket", "error", err)

	return nil, tracing.Error(span, codes.Internal, fmt.Errorf("failed to delete bucket: %w", err))
}

// DriverGrantBucketAccess call grants access to an account.
// The account_name in the request shall be used as a unique identifier to create credentials.
//
// NOTE: this call needs to be idempotent.
// The account_id returned in the response will be used as the unique identifier for deleting this access when calling DriverRevokeBucketAccess.
// The returned secret does not need to be the same each call to achieve idempotency.
func (s *Server) DriverGrantBucketAccess(ctx context.Context, req *cosi.DriverGrantBucketAccessRequest) (*cosi.DriverGrantBucketAccessResponse, error) {
	ctx, span := s.init(ctx, "DriverGrantBucketAccess")
	defer span.End()

	var (
		client linodeclient.Client = s.client
		err    error
	)

	if ref, ok := req.GetParameters()[ParamSecretRef]; ok {
		client, err = s.scopedClient(ctx, ref)
		if err != nil {
			return nil, tracing.Error(span, codes.Internal, err)
		}
	}

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

	span.SetAttributes(
		attribute.String(KeyBucketID, req.GetBucketId()),
		attribute.String(KeyBucketRegion, region),
		attribute.String(KeyBucketLabel, label),
		attribute.String(KeyBucketAccessName, name),
		attribute.String(KeyBucketAccessAuth, auth.String()),
		attribute.String(KeyBucketAccessPermissions, string(perms)),
	)

	if auth != cosi.AuthenticationType_Key {
		log.ErrorContext(ctx, "unsupported authentication type")

		return nil, tracing.Error(span, codes.InvalidArgument, fmt.Errorf("%w: %s", ErrUnsuportedAuth, auth.String()))
	} else if perms != ParamPermissionsValueReadOnly && perms != ParamPermissionsValueReadWrite {
		log.ErrorContext(ctx, "unknown permissions")

		return nil, tracing.Error(span, codes.InvalidArgument, fmt.Errorf("%w: %s", ErrUnknownPermsissions, perms))
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

	log.InfoContext(ctx, "creating object storage key")

	key, err := client.CreateObjectStorageKey(ctx, opts)
	if err != nil {
		log.ErrorContext(ctx, "failed to create object storage key", "error", err)
		return nil, tracing.Error(span, codes.InvalidArgument, fmt.Errorf("failed to create object storage key: %w", err))
	}

	log.InfoContext(ctx, "object storage key created")

	return &cosi.DriverGrantBucketAccessResponse{
		AccountId:   fmt.Sprintf("%d", key.ID),
		Credentials: credentials(region, label, key.AccessKey, key.SecretKey),
	}, tracing.Error(span, codes.OK, nil)
}

// DriverRevokeBucketAccess call revokes all access to a particular bucket from a principal.
//
// NOTE: this call needs to be idempotent.
func (s *Server) DriverRevokeBucketAccess(ctx context.Context, req *cosi.DriverRevokeBucketAccessRequest) (*cosi.DriverRevokeBucketAccessResponse, error) {
	ctx, span := s.init(ctx, "DriverRevokeBucketAccess")
	defer span.End()

	// TODO: requires COSI v1alpha2
	client := s.client
	// var (
	// 	client linodeclient.Client = s.client
	// 	err    error
	// )
	//
	// if ref, ok := req.GetParameters()[ParamSecretRef]; ok {
	// 	client, err = s.scopedClient(ctx, ref)
	// 	if err != nil {
	// 		return nil, tracing.Error(span, codes.Internal, err)
	// 	}
	// }

	region, label := parseBucketID(req.GetBucketId())
	id, err := strconv.Atoi(req.GetAccountId())

	log := s.logAttr(
		slog.String(KeyBucketID, req.GetBucketId()),
		slog.String(KeyBucketAccessIDRaw, req.GetBucketId()),
		slog.String(KeyBucketRegion, region),
		slog.String(KeyBucketLabel, label),
		slog.Int(KeyBucketAccessID, id),
	).WithGroup("DriverRevokeBucketAccess")

	span.SetAttributes(
		attribute.String(KeyBucketID, req.GetBucketId()),
		attribute.String(KeyBucketAccessIDRaw, req.GetBucketId()),
		attribute.String(KeyBucketRegion, region),
		attribute.String(KeyBucketLabel, label),
		attribute.Int(KeyBucketAccessID, id),
	)

	if err != nil {
		log.ErrorContext(ctx, "invalid account id", "error", err)
		return nil, tracing.Error(span, codes.InvalidArgument, fmt.Errorf("account id is invalid: %w", err))
	}

	err = client.DeleteObjectStorageKey(ctx, id)
	if err == nil || errors.Is(err, ErrNotFound) {
		log.InfoContext(ctx, "key deleted")
		return &cosi.DriverRevokeBucketAccessResponse{}, tracing.Error(span, codes.OK, nil, "key deleted")
	}

	log.ErrorContext(ctx, "failed to delete key", "error", err)

	return nil, tracing.Error(span, codes.Internal, fmt.Errorf("failed to delete key: %w", err))
}

func (s *Server) scopedClient(ctx context.Context, secretRef string) (linodeclient.Client, error) {
	const namespacedName = 2

	if s.kubeclient == nil || secretRef == "" {
		return s.client, nil
	}

	ref := strings.Split(secretRef, "/")
	if len(ref) != namespacedName {
		return nil, ErrInvalidSecretReference
	}

	secret := v1.Secret{}

	if err := s.kubeclient.Get(ctx, types.NamespacedName{
		Namespace: ref[0],
		Name:      ref[1],
	}, &secret); err != nil {
		return nil, fmt.Errorf("unable to obtain secret %s: %w", secretRef, err)
	}

	var errs error

	token, err := stringFromSecret(&secret, LinodeTokenKey, true)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	apiURL, err := stringFromSecret(&secret, LinodeAPIURLKey, true)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	apiVersion, err := stringFromSecret(&secret, LinodeAPIVersionKey, true)
	if err != nil {
		errs = errors.Join(errs, err)
	}

	if errs != nil {
		return nil, errs
	}

	debug, _ := boolFromSecret(&secret, LinodeDebugKey, false)

	client, err := linodeclient.NewLinodeClient(
		token,
		version.UserAgent(),
		apiURL,
		apiVersion,
	)
	if err != nil {
		return nil, err
	}

	client.SetDebug(debug)

	// client.SetRootCertificateFromString() // TODO: set CA dynamically

	return client, nil
}
