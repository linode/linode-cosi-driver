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

package identity

import (
	"context"
	"errors"

	cosi "sigs.k8s.io/container-object-storage-interface-spec"
)

var ErrNameEmpty = errors.New("driver name is empty")

// Server implements cosi.IdentityServer interface.
type Server struct {
	name string
}

// Interface guards.
var _ cosi.IdentityServer = (*Server)(nil)

// New returns identitu.Server with name set to the "name" argument.
func New(driverName string) (*Server, error) {
	if driverName == "" {
		return nil, ErrNameEmpty
	}

	srv := &Server{
		name: driverName,
	}

	return srv, nil
}

// DriverGetInfo call is meant to retrieve the unique provisioner Identity.
func (s *Server) DriverGetInfo(_ context.Context, _ *cosi.DriverGetInfoRequest) (*cosi.DriverGetInfoResponse, error) {
	return &cosi.DriverGetInfoResponse{
		Name: s.name,
	}, nil
}
