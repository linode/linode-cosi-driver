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
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	cosi "sigs.k8s.io/container-object-storage-interface/proto"
)

const maxBucketLabelLength = 63

var bucketNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*[a-z0-9]$`)

// DriverGenerateBucketId allocates a deterministic identity without creating a backend bucket.
func (s *Server) DriverGenerateBucketId(_ context.Context, req *cosi.DriverGenerateBucketIdRequest) (*cosi.DriverGenerateBucketIdResponse, error) {
	if err := validateProtocols(req.GetProtocols()); err != nil {
		return nil, err
	}
	region := req.GetParameters()[ParamRegion]
	if region == "" || strings.ContainsAny(region, "/ ") {
		return nil, status.Error(codes.InvalidArgument, "a valid region is required")
	}
	name := req.GetName()
	if len(name) < 3 || !bucketNamePattern.MatchString(name) {
		return nil, status.Error(codes.InvalidArgument, "bucket name must be a lowercase DNS name")
	}
	if len(name) > maxBucketLabelLength {
		digest := sha256.Sum256([]byte(name))
		name = fmt.Sprintf("cosi-%x", digest)[:maxBucketLabelLength]
	}
	cleanup := ParamCleanupValue(req.GetParameters()[ParamCleanup])
	if cleanup != "" && !cleanup.Force() {
		return nil, status.Error(codes.InvalidArgument, "unsupported bucket cleanup policy")
	}
	return &cosi.DriverGenerateBucketIdResponse{BucketId: bucketID(region, name, cleanup)}, nil
}

// DriverGetBucket discovers an existing backend bucket without creating it.
func (s *Server) DriverGetBucket(ctx context.Context, req *cosi.DriverGetBucketRequest) (*cosi.DriverGetBucketResponse, error) {
	if err := validateProtocols(req.GetProtocols()); err != nil {
		return nil, err
	}
	region, label, _, err := parseBucketID(req.GetBucketId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	bucket, err := s.client.GetObjectStorageBucket(ctx, region, label)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, status.Error(codes.NotFound, "bucket does not exist")
		}
		return nil, status.Error(codes.Internal, fmt.Sprintf("get bucket: %v", err))
	}
	endpoint, err := s.endpointForBucket(ctx, region, bucket)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("resolve bucket endpoint: %v", err))
	}
	return &cosi.DriverGetBucketResponse{Protocols: bucketInfo(region, label, endpoint)}, nil
}

func validateProtocol(protocol *cosi.ObjectProtocol) error {
	if protocol.GetType() != cosi.ObjectProtocol_S3 {
		return status.Error(codes.InvalidArgument, "only S3 is supported")
	}
	return nil
}

func validateProtocols(protocols []*cosi.ObjectProtocol) error {
	for _, protocol := range protocols {
		if err := validateProtocol(protocol); err != nil {
			return err
		}
	}
	return nil
}

func validateAccessRequest(req *cosi.DriverGrantBucketAccessRequest) error {
	if err := validateProtocol(req.GetProtocol()); err != nil {
		return err
	}
	if req.GetAuthenticationType().GetType() != cosi.AuthenticationType_KEY {
		return status.Error(codes.InvalidArgument, "only Key authentication is supported")
	}
	if len(req.GetBuckets()) > 1 {
		return status.Error(codes.OutOfRange, "only single-bucket access is supported")
	}
	if len(req.GetBuckets()) == 0 || req.GetAccountName() == "" {
		return status.Error(codes.InvalidArgument, "account name and one accessed bucket are required")
	}
	return nil
}

func permissionsForAccess(bucket *cosi.DriverGrantBucketAccessRequest_AccessedBucket) (ParamPermissionsValue, error) {
	requested := cosi.AccessMode_ANY
	for _, mode := range []*cosi.AccessMode{bucket.GetObjectDataAccessMode(), bucket.GetObjectMetadataAccessMode(), bucket.GetBucketMetadataAccessMode()} {
		if mode == nil {
			continue
		}
		value := mode.GetMode()
		if value == cosi.AccessMode_ANY {
			continue
		}
		if value != cosi.AccessMode_READ_ONLY && value != cosi.AccessMode_READ_WRITE {
			return "", status.Error(codes.InvalidArgument, "only ReadOnly and ReadWrite access modes are supported")
		}
		if requested != cosi.AccessMode_ANY && requested != value {
			return "", status.Error(codes.InvalidArgument, "Linode keys cannot apply different permissions to data and metadata")
		}
		requested = value
	}
	if requested == cosi.AccessMode_ANY {
		return "", status.Error(codes.InvalidArgument, "at least one access mode is required")
	}
	if requested == cosi.AccessMode_READ_WRITE {
		return ParamPermissionsValueReadWrite, nil
	}
	return ParamPermissionsValueReadOnly, nil
}
