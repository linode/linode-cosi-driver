// Copyright 2023-2025 Akamai Technologies, Inc.
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
	prefix           = "cosi.linode.com/v1/"
	ParamACL         = prefix + "acl"
	ParamCORS        = prefix + "cors"
	ParamPermissions = prefix + "permissions"
	ParamPolicy      = prefix + "policy"
	ParamRegion      = prefix + "region"
	// ParamLinodeTokenSecretName points to a Secret containing LINODE_TOKEN.
	ParamLinodeTokenSecretName = prefix + "linode-token-secret-name"
	// ParamLinodeTokenSecretNamespace is the namespace of ParamLinodeTokenSecretName.
	ParamLinodeTokenSecretNamespace = prefix + "linode-token-secret-namespace"
	// AnnotationLinodeTokenSecretName allows BucketClaim annotations to specify a Secret containing LINODE_TOKEN.
	AnnotationLinodeTokenSecretName = ParamLinodeTokenSecretName
	// AnnotationLinodeTokenSecretNamespace is the namespace of AnnotationLinodeTokenSecretName.
	AnnotationLinodeTokenSecretNamespace = ParamLinodeTokenSecretNamespace
)

// TODO(v1alpha2): add the cleanup:
//
//	const ParamCleanup = prefix + "cleanup"
//
//	type ParamCleanupValue string
//
//	const (
//		ParamCleanupForce ParamCleanupValue = "force"
//	)
//
//	func (v ParamCleanupValue) Force() bool {
//		return v == ParamCleanupForce
//	}

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

var (
	ErrNotFound            = linodego.Error{Code: http.StatusNotFound}
	ErrUnsuportedAuth      = errors.New("unsupported authentication schema")
	ErrMissingRegion       = errors.New("region was not provided")
	ErrUnknownPermsissions = errors.New("unknown permissions")
	ErrValidationError     = errors.New("required value cannot be empty")
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
