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
	"errors"
	"net/http"

	"github.com/linode/linodego"
)

const (
	ParamRegion      = "cosi.linode.com/v1/region"
	ParamACL         = "cosi.linode.com/v1/acl"
	ParamCORS        = "cosi.linode.com/v1/cors"
	ParamPermissions = "cosi.linode.com/v1/permissions"
	ParamSecretRef   = "cosi.linode.com/v1/secretRef" //nolint:gosec // this is key under which secret reference is placed
)

type ParamCORSValue string

const (
	ParamCORSValueEnabled  ParamCORSValue = "enabled"
	ParamCORSValueDisabled ParamCORSValue = "disabled"
)

func (v ParamCORSValue) Bool() bool {
	return v == ParamCORSValueEnabled
}

func (v ParamCORSValue) BoolP() *bool {
	p := v == ParamCORSValueEnabled
	return &p
}

type ParamPermissionsValue string

const (
	ParamPermissionsValueReadOnly  ParamPermissionsValue = "read_only"
	ParamPermissionsValueReadWrite ParamPermissionsValue = "read_write"
)

const (
	S3                      = "s3"
	S3Region                = "region"
	S3Endpoint              = "endpoint"
	S3SecretAccessKeyID     = "accessKeyID"
	S3SecretAccessSecretKey = "accessSecretKey"
)

const (
	LinodeTokenKey      = "LINODE_TOKEN"
	LinodeAPIURLKey     = "LINODE_API_URL"
	LinodeAPIVersionKey = "LINODE_API_VERSION"
	LinodeDebugKey      = "LINODE_DEBUG"
	// LinodeCAKey         = "LINODE_CA" // TODO: enable setting Linode CA from string in Linode Go.
)

var (
	ErrNotFound               = linodego.Error{Code: http.StatusNotFound}
	ErrBucketExists           = errors.New("bucket exists with different parameters")
	ErrUnsuportedAuth         = errors.New("unsupported authentication schema")
	ErrMissingRegion          = errors.New("region was not provided")
	ErrUnknownPermsissions    = errors.New("unknown permissions")
	ErrInvalidSecretReference = errors.New("invalid secret reference")
	ErrInvalidSecret          = errors.New("invalid secret")
)

const (
	KeyBucketID                = "bucket.id"
	KeyBucketLabel             = "bucket.label"
	KeyBucketRegion            = "bucket.region"
	KeyBucketCreationTimestamp = "bucket.created_at"
	KeyBucketACL               = "bucket.acl"
	KeyBucketCORS              = "bucket.cors_enabled"
	KeyBucketAccessIDRaw       = "bucket.access.id_raw"
	KeyBucketAccessID          = "bucket.access.id"
	KeyBucketAccessName        = "bucket.access.name"
	KeyBucketAccessAuth        = "bucket.access.auth"
	KeyBucketAccessPermissions = "bucket.access.permissions"
)
