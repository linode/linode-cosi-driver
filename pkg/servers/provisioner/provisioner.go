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
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/linode/linodego"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	"github.com/linode/linode-cosi-driver/pkg/linodeclient/cache"
	"github.com/linode/linode-cosi-driver/pkg/s3"
)

// Server implements cosi.ProvisionerServer interface.
type Server struct {
	log  *slog.Logger
	once sync.Once

	client          linodeclient.Client
	cache           cache.Cache
	s3cli           s3.Client
	s3SSL           bool
	perBucketTokens bool
	kubeClient      kubernetes.Interface
	dynClient       dynamic.Interface
	userAgent       string
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
	perBucketTokens bool,
	kubeClient kubernetes.Interface,
	dynClient dynamic.Interface,
	userAgent string,
) (*Server, error) {
	srv := &Server{
		log:             logger,
		client:          client,
		cache:           cache,
		s3cli:           s3cli,
		s3SSL:           s3SSL,
		perBucketTokens: perBucketTokens,
		kubeClient:      kubeClient,
		dynClient:       dynClient,
		userAgent:       userAgent,
	}

	return srv, nil
}

func (s *Server) s3ClientForBucket(ctx context.Context, client linodeclient.Client, region, label string) (s3.Client, func(context.Context) error, error) {
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

	key, err := client.CreateObjectStorageKey(ctx, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create object storage key for bucket: %w", err)
	}

	cleanup := func(cctx context.Context) error {
		return client.DeleteObjectStorageKey(cctx, key.ID)
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

func (s *Server) clientForRequest(ctx context.Context, bucketName, bucketID string, params map[string]string) (linodeclient.Client, error) {
	token, err := s.tokenForRequest(ctx, bucketName, bucketID, params)
	if err != nil {
		return nil, err
	}
	if token == "" {
		return s.client, nil
	}
	if s.userAgent == "" {
		return nil, fmt.Errorf("user agent not set for token-based client")
	}
	return linodeclient.NewLinodeClientWithToken(s.userAgent, token)
}

func (s *Server) tokenForRequest(ctx context.Context, bucketName, bucketID string, params map[string]string) (string, error) {
	if !s.perBucketTokens {
		return "", nil
	}
	name := ""
	namespace := ""
	if params != nil {
		name = params[ParamLinodeTokenSecretName]
		namespace = params[ParamLinodeTokenSecretNamespace]
	}
	if name != "" {
		return s.tokenFromSecret(ctx, namespace, name)
	}
	if bucketName != "" {
		return s.tokenFromBucketClaimName(ctx, bucketName)
	}
	if bucketID == "" {
		return "", nil
	}
	return s.tokenFromBucketID(ctx, bucketID)
}

func (s *Server) tokenFromSecret(ctx context.Context, namespace, name string) (string, error) {
	if name == "" {
		return "", nil
	}
	if s.kubeClient == nil {
		return "", fmt.Errorf("kubernetes client not configured for secret lookup")
	}
	if namespace == "" {
		return "", fmt.Errorf("secret namespace is required")
	}
	secret, err := s.kubeClient.CoreV1().Secrets(namespace).Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get linode token secret %s/%s: %w", namespace, name, err)
	}
	token := strings.TrimSpace(string(secret.Data[linodeTokenSecretKey]))
	if token == "" {
		return "", fmt.Errorf("secret %s/%s missing %s", namespace, name, linodeTokenSecretKey)
	}
	return token, nil
}

func (s *Server) tokenFromBucketID(ctx context.Context, bucketID string) (string, error) {
	if s.dynClient == nil {
		return "", nil
	}
	_, label := parseBucketID(bucketID)
	if label != "" {
		token, err := s.tokenFromBucketClaimName(ctx, label)
		if err != nil || token != "" {
			return token, err
		}
	}
	buckets, err := s.dynClient.Resource(bucketGVR).List(ctx, v1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list buckets: %w", err)
	}
	for _, item := range buckets.Items {
		id, _, _ := unstructured.NestedString(item.Object, "status", "bucketID")
		if id != bucketID {
			continue
		}
		className, _, _ := unstructured.NestedString(item.Object, "spec", "bucketClassName")
		if className == "" {
			return "", nil
		}
		return s.tokenFromBucketClass(ctx, className)
	}
	return "", nil
}

func (s *Server) tokenFromBucketClaimName(ctx context.Context, claimName string) (string, error) {
	if claimName == "" || s.dynClient == nil {
		return "", nil
	}
	list, err := s.dynClient.Resource(bucketClaimGVR).List(ctx, v1.ListOptions{
		FieldSelector: "metadata.name=" + claimName,
	})
	if err != nil {
		return "", fmt.Errorf("list bucketclaims: %w", err)
	}
	if len(list.Items) == 0 {
		return "", nil
	}
	if len(list.Items) > 1 {
		return "", fmt.Errorf("multiple bucketclaims named %s", claimName)
	}
	item := list.Items[0]
	annotations, _, _ := unstructured.NestedStringMap(item.Object, "metadata", "annotations")
	name := annotations[AnnotationLinodeTokenSecretName]
	if name == "" {
		return "", nil
	}
	namespace := annotations[AnnotationLinodeTokenSecretNamespace]
	if namespace == "" {
		namespace, _, _ = unstructured.NestedString(item.Object, "metadata", "namespace")
	}
	return s.tokenFromSecret(ctx, namespace, name)
}

func (s *Server) tokenFromBucketClass(ctx context.Context, className string) (string, error) {
	if s.dynClient == nil {
		return "", nil
	}
	classObj, err := s.dynClient.Resource(bucketClassGVR).Get(ctx, className, v1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get bucketclass %s: %w", className, err)
	}
	params, _, _ := unstructured.NestedStringMap(classObj.Object, "parameters")
	name := params[ParamLinodeTokenSecretName]
	namespace := params[ParamLinodeTokenSecretNamespace]
	if name == "" {
		return "", nil
	}
	return s.tokenFromSecret(ctx, namespace, name)
}

const keyCleanupTimeout = 3 * time.Second

const linodeTokenSecretKey = "LINODE_TOKEN"

var (
	bucketGVR = schema.GroupVersionResource{Group: "objectstorage.k8s.io", Version: "v1alpha1", Resource: "buckets"}
	bucketClaimGVR = schema.GroupVersionResource{Group: "objectstorage.k8s.io", Version: "v1alpha1", Resource: "bucketclaims"}
	bucketClassGVR = schema.GroupVersionResource{Group: "objectstorage.k8s.io", Version: "v1alpha1", Resource: "bucketclasses"}
)

func cleanupWithTimeout(ctx context.Context, log *slog.Logger, cleanup func(context.Context) error) {
	cctx, cancel := context.WithTimeout(ctx, keyCleanupTimeout)
	defer cancel()

	if err := cleanup(cctx); err != nil {
		log.ErrorContext(ctx, "Failed to cleanup bucket-scoped credentials", "error", err)
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
	client, err := s.clientForRequest(ctx, label, "", req.GetParameters())
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to resolve linode client: %v", err))
	}

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

	policy, err := s.buildBucketPolicy(policyTemplate, label)
	if err != nil {
		log.ErrorContext(ctx, "Failed to generate bucket policy", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to generate bucket policy: %v", err))
	}

	bucket, err := client.GetObjectStorageBucket(ctx, region, label)
	if err != nil && !errors.Is(err, ErrNotFound) {
		log.ErrorContext(ctx, "Failed to check if bucket exists", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to check if bucket exists: %v", err))
	}

	if bucket == nil {
		// Create the bucket if it doesn't exist, then apply policy if provided.
		return s.createBucketAndApplyPolicy(ctx, client, log, region, label, acl, cors, policy)
	}

	// Bucket exists: validate parameters and re-apply policy for idempotency.
	return s.ensureExistingBucket(ctx, client, log, region, label, acl, cors, policy)
}

func (s *Server) buildBucketPolicy(policyTemplate, label string) (string, error) {
	if policyTemplate != "" {
		return "", nil
	}
	return s3.ApplyTemplate(policyTemplate, s3.PolicyTemplateParams{
		BucketName: label,
	})
}

func (s *Server) createBucketAndApplyPolicy(
	ctx context.Context,
	client linodeclient.Client,
	log *slog.Logger,
	region, label string,
	acl linodego.ObjectStorageACL,
	cors ParamCORSValue,
	policy string,
) (*cosi.DriverCreateBucketResponse, error) {
	opts := linodego.ObjectStorageBucketCreateOptions{
		Region:      region,
		Label:       label,
		ACL:         acl,
		CorsEnabled: cors.BoolP(),
	}

	log.InfoContext(ctx, "Creating bucket")

	bucket, err := client.CreateObjectStorageBucket(ctx, opts)
	if err != nil {
		log.ErrorContext(ctx, "Failed to create bucket", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to create bucket: %v", err))
	}

	log.InfoContext(ctx, "Bucket created")

	if policy != "" {
		log.InfoContext(ctx, "Updating policy")

		if err := s.applyBucketPolicy(ctx, client, log, region, bucket.Label, policy); err != nil {
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
	client linodeclient.Client,
	log *slog.Logger,
	region, label string,
	acl linodego.ObjectStorageACL,
	cors ParamCORSValue,
	policy string,
) (*cosi.DriverCreateBucketResponse, error) {
	access, err := client.GetObjectStorageBucketAccess(ctx, region, label)
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

	// Comparing policies is expensive and hard. If every other parameter is equal,
	// we assume that bucket is valid, and the policy will be applied.
	if err := s.applyBucketPolicy(ctx, client, log, region, label, policy); err != nil {
		log.ErrorContext(ctx, "Failed to set bucket policy", "error", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to set bucket policy: %v", err))
	}

	log.InfoContext(ctx, "Bucket exists")

	return &cosi.DriverCreateBucketResponse{
		BucketId:   region + "/" + label,
		BucketInfo: bucketInfo(region),
	}, status.Error(codes.OK, "bucket exists")
}

func (s *Server) applyBucketPolicy(ctx context.Context, client linodeclient.Client, log *slog.Logger, region, label, policy string) error {
	s3cli, cleanup, err := s.s3ClientForBucket(ctx, client, region, label)
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
	client, err := s.clientForRequest(ctx, label, req.GetBucketId(), nil)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to resolve linode client: %v", err))
	}
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
		s3cli, keyCleanup, err := s.s3ClientForBucket(ctx, client, region, label)
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

	err = client.DeleteObjectStorageBucket(ctx, region, label)
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
	client, err := s.clientForRequest(ctx, label, req.GetBucketId(), req.GetParameters())
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to resolve linode client: %v", err))
	}

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

	key, err := client.CreateObjectStorageKey(ctx, opts)
	if err != nil {
		log.ErrorContext(ctx, "Failed to create object storage key", "error", err)
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to create object storage key: %v", err))
	}

	log.InfoContext(ctx, "Object storage key created")

	endpoint, ok := s.cache.Get(region)
	if !ok || endpoint == "" {
		log.ErrorContext(ctx, "Failed to get endpoint for region", "region", region)
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to get endpoint for region: %v", region))
	}

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
	client, clientErr := s.clientForRequest(ctx, label, req.GetBucketId(), nil)
	if clientErr != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("failed to resolve linode client: %v", clientErr))
	}

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

	err = client.DeleteObjectStorageKey(ctx, id)
	if err == nil || errors.Is(err, ErrNotFound) {
		log.InfoContext(ctx, "Key deleted")
		return &cosi.DriverRevokeBucketAccessResponse{}, status.Error(codes.OK, "key deleted")
	}

	log.ErrorContext(ctx, "Failed to delete key", "error", err)

	return nil, status.Error(codes.Internal, fmt.Sprintf("failed to delete key: %v", err))
}
