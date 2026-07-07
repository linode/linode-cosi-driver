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

package provisioner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linode/linodego"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient/cache"
	"github.com/linode/linode-cosi-driver/pkg/s3"
)

// Server implements cosi.ProvisionerServer interface.
type Server struct {
	log  *slog.Logger
	once sync.Once

	client linodeclient.Client
	cache  cache.Cache
	s3cli  s3.Client
	s3SSL  bool
}

// Interface guards.
var _ cosi.ProvisionerServer = (*Server)(nil)

// New returns provisioner.Server with default values.
func New(
	logger *slog.Logger,
	client linodeclient.Client,
	cache cache.Cache,
	s3cli s3.Client,
	s3SSL bool,
) (*Server, error) {
	srv := &Server{
		log:    logger,
		client: client,
		cache:  cache,
		s3cli:  s3cli,
		s3SSL:  s3SSL,
	}

	return srv, nil
}

func (s *Server) s3ClientForBucket(ctx context.Context, region, label string) (s3.Client, func(context.Context) error, error) {
	if s.s3cli != nil {
		return s.s3cli, func(context.Context) error { return nil }, nil
	}

	keyLabel := fmt.Sprintf("cosi-bucket-%s", uuid.NewString())
	opts := linodego.ObjectStorageKeyCreateOptions{
		Label: keyLabel,
		BucketAccess: &[]linodego.ObjectStorageKeyBucketAccess{
			{
				Region:      region,
				BucketName:  label,
				Permissions: string(ParamPermissionsValueReadWrite),
			},
		},
	}

	key, err := s.client.CreateObjectStorageKey(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create object storage key for bucket: %w", err)
	}

	cleanup := func(cctx context.Context) error {
		return s.client.DeleteObjectStorageKey(cctx, key.ID)
	}

	return s3.New(s.cache, key.AccessKey, key.SecretKey, s.s3SSL), cleanup, nil
}

func (s *Server) s3ClientForPolicy(ctx context.Context, region string) (s3.Client, func(context.Context) error, error) {
	if s.s3cli != nil {
		return s.s3cli, func(context.Context) error { return nil }, nil
	}

	key, cleanup, err := linodeclient.NewEphemeralS3Credentials(ctx, s.logAttr(), s.client, region)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create object storage key for policy updates: %w", err)
	}

	return s3.New(s.cache, key.AccessKey, key.SecretKey, s.s3SSL), cleanup, nil
}

func (s *Server) logAttr(attr ...slog.Attr) *slog.Logger {
	s.once.Do(func() {
		if s.log == nil {
			s.log = slog.Default()
		}
	})

	return slog.New(s.log.Handler().WithAttrs(attr))
}

const keyCleanupTimeout = 30 * time.Second

func cleanupWithTimeout(ctx context.Context, log *slog.Logger, cleanup func(context.Context) error) {
	cctx, cancel := context.WithTimeout(ctx, keyCleanupTimeout)
	defer cancel()

	if err := cleanup(cctx); err != nil {
		log.ErrorContext(ctx, "Failed to cleanup temporary S3 credentials", "error", err)
	}
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
	policyTemplate := req.GetParameters()[ParamPolicy]

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

	endpointType, err := s.selectEndpointType(ctx, region, req.GetParameters())
	if err != nil {
		log.ErrorContext(ctx, "Failed to select endpoint", "error", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if endpointType != "" {
		log = log.With(slog.String(KeyBucketEndpointType, string(endpointType)))
	}

	policy, err := s.buildBucketPolicy(policyTemplate, label)
	if err != nil {
		log.ErrorContext(ctx, "Failed to generate bucket policy", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to generate bucket policy: %v", err))
	}

	bucket, err := s.client.GetObjectStorageBucket(ctx, region, label)
	if err != nil && !errors.Is(err, ErrNotFound) {
		log.ErrorContext(ctx, "Failed to check if bucket exists", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to check if bucket exists: %v", err))
	}

	if bucket == nil {
		// Create the bucket if it doesn't exist, then apply policy if provided.
		return s.createBucketAndApplyPolicy(ctx, log, region, label, endpointType, acl, cors, policy)
	}

	// Bucket exists: validate parameters and re-apply policy for idempotency.
	return s.ensureExistingBucket(ctx, log, bucket, region, label, endpointType, acl, cors, policy)
}

func (s *Server) buildBucketPolicy(policyTemplate, label string) (string, error) {
	if policyTemplate == "" {
		return "", nil
	}
	return s3.ApplyTemplate(policyTemplate, s3.PolicyTemplateParams{
		BucketName: label,
	})
}

func (s *Server) createBucketAndApplyPolicy(
	ctx context.Context,
	log *slog.Logger,
	region, label string,
	endpointType linodego.ObjectStorageEndpointType,
	acl linodego.ObjectStorageACL,
	cors ParamCORSValue,
	policy string,
) (*cosi.DriverCreateBucketResponse, error) {
	opts := linodego.ObjectStorageBucketCreateOptions{
		Region: region,
		Label:  label,
		ACL:    acl,
	}
	if endpointType != "" {
		opts.EndpointType = endpointType
	}
	if cors.Bool() {
		opts.CorsEnabled = cors.BoolP()
	}

	log.InfoContext(ctx, "Creating bucket")

	bucket, err := s.client.CreateObjectStorageBucket(ctx, opts)
	if err != nil {
		log.ErrorContext(ctx, "Failed to create bucket", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create bucket: %v", err))
	}

	log.InfoContext(ctx, "Bucket created")

	if policy != "" {
		log.InfoContext(ctx, "Updating policy")

		if err := s.applyBucketPolicy(ctx, log, region, bucket.Label, policy); err != nil {
			log.ErrorContext(ctx, "Failed to set bucket policy", "error", err)
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to set bucket policy: %v", err))
		}
	}

	return &cosi.DriverCreateBucketResponse{
		BucketId:   bucket.Region + "/" + bucket.Label,
		BucketInfo: bucketInfo(bucket.Region),
	}, status.Error(codes.OK, "bucket created")
}

func (s *Server) ensureExistingBucket(
	ctx context.Context,
	log *slog.Logger,
	bucket *linodego.ObjectStorageBucket,
	region, label string,
	endpointType linodego.ObjectStorageEndpointType,
	acl linodego.ObjectStorageACL,
	cors ParamCORSValue,
	policy string,
) (*cosi.DriverCreateBucketResponse, error) {
	access, err := s.client.GetObjectStorageBucketAccess(ctx, region, label)
	if err != nil {
		log.ErrorContext(ctx, "Failed to check bucket access", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to check bucket access: %v", err))
	}

	if (endpointType != "" && bucket.EndpointType != endpointType) ||
		access.ACL != acl ||
		access.CorsEnabled != cors.Bool() {
		log.ErrorContext(ctx, "Bucket with different parameters already exists",
			"existing_"+KeyBucketEndpointType, bucket.EndpointType,
			"existing_"+KeyBucketACL, access.ACL,
			"existing_"+KeyBucketCORS, access.CorsEnabled,
		)

		return nil, status.Error(codes.AlreadyExists, "bucket exists with different parameters")
	}

	// Comparing policies is expensive and hard. If every other parameter is equal,
	// we assume that bucket is valid, and apply policy only when one was provided.
	if policy != "" {
		if err := s.applyBucketPolicy(ctx, log, region, label, policy); err != nil {
			log.ErrorContext(ctx, "Failed to set bucket policy", "error", err)
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to set bucket policy: %v", err))
		}
	}

	log.InfoContext(ctx, "Bucket exists")

	return &cosi.DriverCreateBucketResponse{
		BucketId:   region + "/" + label,
		BucketInfo: bucketInfo(region),
	}, status.Error(codes.OK, "bucket exists")
}

func (s *Server) selectEndpointType(
	ctx context.Context,
	region string,
	params map[string]string,
) (linodego.ObjectStorageEndpointType, error) {
	endpointTypes, err := parseEndpointTypePreference(params)
	if err != nil {
		return "", err
	}

	if err := validateExplicitEndpointTypeParams(params); err != nil {
		return "", err
	}

	if len(endpointTypes) == 0 && !corsEnabled(params) {
		return "", nil
	}

	endpoints, err := s.client.ListObjectStorageEndpoints(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("list object storage endpoints: %w", err)
	}

	endpoint, ok := selectEndpoint(endpoints, region, endpointTypes, params)
	if ok {
		return endpoint.EndpointType, nil
	}

	if len(endpointTypes) > 0 {
		return "", fmt.Errorf("no preferred object storage endpoint type is available for region: %s", region)
	}

	return "", fmt.Errorf("no object storage endpoint available for region: %s", region)
}

func selectEndpoint(
	endpoints []linodego.ObjectStorageEndpoint,
	region string,
	endpointTypes []linodego.ObjectStorageEndpointType,
	params map[string]string,
) (linodego.ObjectStorageEndpoint, bool) {
	if len(endpointTypes) == 0 {
		return firstMatchingEndpoint(endpoints, region, params)
	}

	return firstPreferredEndpoint(endpoints, region, endpointTypes, params)
}

func firstPreferredEndpoint(
	endpoints []linodego.ObjectStorageEndpoint,
	region string,
	endpointTypes []linodego.ObjectStorageEndpointType,
	params map[string]string,
) (linodego.ObjectStorageEndpoint, bool) {
	for _, endpointType := range endpointTypes {
		endpoint, ok := firstMatchingEndpointByType(endpoints, region, endpointType, params)
		if ok {
			return endpoint, true
		}
	}

	return linodego.ObjectStorageEndpoint{}, false
}

func firstMatchingEndpointByType(
	endpoints []linodego.ObjectStorageEndpoint,
	region string,
	endpointType linodego.ObjectStorageEndpointType,
	params map[string]string,
) (linodego.ObjectStorageEndpoint, bool) {
	for _, endpoint := range endpoints {
		if endpoint.EndpointType != endpointType ||
			!endpointMatchesParams(endpoint, region, params) {
			continue
		}

		return endpoint, true
	}

	return linodego.ObjectStorageEndpoint{}, false
}

func firstMatchingEndpoint(
	endpoints []linodego.ObjectStorageEndpoint,
	region string,
	params map[string]string,
) (linodego.ObjectStorageEndpoint, bool) {
	for _, endpoint := range endpoints {
		if !endpointMatchesParams(endpoint, region, params) {
			continue
		}

		return endpoint, true
	}

	return linodego.ObjectStorageEndpoint{}, false
}

func (s *Server) endpointForType(
	ctx context.Context,
	region string,
	endpointType linodego.ObjectStorageEndpointType,
) (linodego.ObjectStorageEndpoint, error) {
	endpoints, err := s.client.ListObjectStorageEndpoints(ctx, nil)
	if err != nil {
		return linodego.ObjectStorageEndpoint{}, fmt.Errorf("list object storage endpoints: %w", err)
	}

	for _, endpoint := range endpoints {
		if endpoint.Region != region ||
			endpoint.S3Endpoint == nil ||
			*endpoint.S3Endpoint == "" ||
			endpoint.EndpointType != endpointType {
			continue
		}

		return endpoint, nil
	}

	return linodego.ObjectStorageEndpoint{}, fmt.Errorf("object storage endpoint type %s is not available for region: %s", endpointType, region)
}

func (s *Server) endpointForBucket(ctx context.Context, region string, bucket *linodego.ObjectStorageBucket) (string, error) {
	if bucket.S3Endpoint != "" {
		return bucket.S3Endpoint, nil
	}

	endpoint, err := s.endpointForType(ctx, region, bucket.EndpointType)
	if err != nil {
		return "", err
	}

	return *endpoint.S3Endpoint, nil
}

func endpointMatchesParams(endpoint linodego.ObjectStorageEndpoint, region string, params map[string]string) bool {
	if endpoint.Region != region ||
		endpoint.S3Endpoint == nil ||
		*endpoint.S3Endpoint == "" {
		return false
	}

	if corsEnabled(params) && !endpointTypeSupportsCORS(endpoint.EndpointType) {
		return false
	}

	return true
}

func validateExplicitEndpointTypeParams(params map[string]string) error {
	endpointType := linodego.ObjectStorageEndpointType(params[ParamEndpointType])
	if endpointType == "" || endpointTypeSupportsCORS(endpointType) || !corsEnabled(params) {
		return nil
	}

	return fmt.Errorf("endpoint type %s does not support CORS", endpointType)
}

func corsEnabled(params map[string]string) bool {
	return ParamCORSValue(params[ParamCORS]).Bool()
}

func endpointTypeSupportsCORS(endpointType linodego.ObjectStorageEndpointType) bool {
	return endpointType != linodego.ObjectStorageEndpointE2 &&
		endpointType != linodego.ObjectStorageEndpointE3
}

func (s *Server) applyBucketPolicy(ctx context.Context, log *slog.Logger, region, label, policy string) error {
	if policy == "" {
		return nil
	}

	s3cli, cleanup, err := s.s3ClientForPolicy(ctx, region)
	if err != nil {
		return err
	}
	defer cleanupWithTimeout(ctx, log, cleanup)

	return s3cli.SetBucketPolicy(ctx, region, label, policy)
}

// DriverDeleteBucket call is made to delete the bucket in the backend.
//
// NOTE: this call needs to be idempotent.
// If the bucket has already been deleted, then no error should be returned.
func (s *Server) DriverDeleteBucket(ctx context.Context, req *cosi.DriverDeleteBucketRequest) (*cosi.DriverDeleteBucketResponse, error) {
	region, label := parseBucketID(req.GetBucketId())
	// TODO(v1alpha2): add the cleanup
	// := ParamCleanupValue(req.GetParameters()[ParamCleanup]).Force()
	cleanup := false

	log := s.logAttr(
		slog.String(KeyBucketID, req.GetBucketId()),
		slog.String(KeyBucketRegion, region),
		slog.String(KeyBucketLabel, label),
	).WithGroup("DriverDeleteBucket")

	log.InfoContext(ctx, "Bucket deletion initiated")

	if cleanup {
		s3cli, keyCleanup, err := s.s3ClientForBucket(ctx, region, label)
		if err != nil {
			log.ErrorContext(ctx, "Failed to create bucket-scoped credentials", "error", err)
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create bucket-scoped credentials: %v", err))
		}
		defer cleanupWithTimeout(ctx, log, keyCleanup)

		err = s3cli.Prune(ctx, region, label)
		if err != nil && !s3.IsNotFound(err) {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to cleanup bucket: %v", err))
		}
	}

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

	bucket, err := s.client.GetObjectStorageBucket(ctx, region, label)
	if err != nil {
		log.ErrorContext(ctx, "Failed to get bucket", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get bucket: %v", err))
	}

	endpoint, err := s.endpointForBucket(ctx, region, bucket)
	if err != nil {
		log.ErrorContext(ctx, "Failed to select endpoint", "error", err)
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	log = log.With(slog.String(KeyBucketEndpointType, string(bucket.EndpointType)))

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
		Credentials: credentials(region, endpoint, label, key.AccessKey, key.SecretKey),
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
