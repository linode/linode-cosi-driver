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
	"log/slog"
	"sync"

	"github.com/linode/linode-cosi-driver/pkg/linodeclient"
	cosi "sigs.k8s.io/container-object-storage-interface-spec"
)

// Server implements cosi.ProvisionerServer interface.
type Server struct {
	log  *slog.Logger
	once sync.Once

	client linodeclient.LinodeClient
}

// Interface guards.
var _ cosi.ProvisionerServer = (*Server)(nil)

// New returns provisioner.Server with default values.
func New(logger *slog.Logger, client linodeclient.LinodeClient) (*Server, error) {
	return &Server{
		log:    logger,
		client: client,
	}, nil
}

// init exists in case someone initializes server with nil logger.
func (s *Server) init() {
	if s.log == nil {
		s.log = slog.Default()
	}
}

// DriverCreateBucket call is made to create the bucket in the backend.
//
// NOTE: this call needs to be idempotent.
//  1. If a bucket that matches both name and parameters already exists, then OK (success) must be returned.
//  2. If a bucket by same name, but different parameters is provided, then the appropriate error code ALREADY_EXISTS must be returned.
func (s *Server) DriverCreateBucket(_ context.Context, _ *cosi.DriverCreateBucketRequest) (*cosi.DriverCreateBucketResponse, error) {
	s.once.Do(s.init)

	panic("FIXME: unimplemented")
}

// DriverDeleteBucket call is made to delete the bucket in the backend.
//
// NOTE: this call needs to be idempotent.
// If the bucket has already been deleted, then no error should be returned.
func (s *Server) DriverDeleteBucket(_ context.Context, _ *cosi.DriverDeleteBucketRequest) (*cosi.DriverDeleteBucketResponse, error) {
	s.once.Do(s.init)

	panic("FIXME: unimplemented")
}

// DriverGrantBucketAccess call grants access to an account.
// The account_name in the request shall be used as a unique identifier to create credentials.
//
// NOTE: this call needs to be idempotent.
// The account_id returned in the response will be used as the unique identifier for deleting this access when calling DriverRevokeBucketAccess.
// The returned secret does not need to be the same each call to achieve idempotency.
func (s *Server) DriverGrantBucketAccess(_ context.Context, _ *cosi.DriverGrantBucketAccessRequest) (*cosi.DriverGrantBucketAccessResponse, error) {
	s.once.Do(s.init)

	panic("FIXME: unimplemented")
}

// DriverRevokeBucketAccess call revokes all access to a particular bucket from a principal.
//
// NOTE: this call needs to be idempotent.
func (s *Server) DriverRevokeBucketAccess(_ context.Context, _ *cosi.DriverRevokeBucketAccessRequest) (*cosi.DriverRevokeBucketAccessResponse, error) {
	s.once.Do(s.init)

	panic("FIXME: unimplemented")
}
