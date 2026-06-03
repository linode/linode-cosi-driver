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
	"fmt"
	"net/http"
	"strings"

	"github.com/linode/linodego"
)

const (
	prefix                      = "cosi.linode.com/v1/"
	ParamACL                    = prefix + "acl"
	ParamCORS                   = prefix + "cors"
	ParamEndpointType           = prefix + "endpoint-type"
	ParamEndpointTypePreference = prefix + "endpoint-type-preference"
	ParamPermissions            = prefix + "permissions"
	ParamPolicy                 = prefix + "policy"
	ParamRegion                 = prefix + "region"
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

func parseEndpointType(params map[string]string) (linodego.ObjectStorageEndpointType, error) {
	endpointType := linodego.ObjectStorageEndpointType(params[ParamEndpointType])
	if endpointType == "" {
		return "", nil
	}

	switch endpointType {
	case linodego.ObjectStorageEndpointE0,
		linodego.ObjectStorageEndpointE1,
		linodego.ObjectStorageEndpointE2,
		linodego.ObjectStorageEndpointE3:
		return endpointType, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownEndpointType, endpointType)
	}
}

func parseEndpointTypePreference(params map[string]string) ([]linodego.ObjectStorageEndpointType, error) {
	if endpointType, err := parseEndpointType(params); err != nil || endpointType != "" {
		return []linodego.ObjectStorageEndpointType{endpointType}, err
	}

	value := strings.TrimSpace(params[ParamEndpointTypePreference])
	if value == "" {
		return nil, nil
	}

	parts := strings.Split(value, ",")
	preferences := make([]linodego.ObjectStorageEndpointType, 0, len(parts))
	for _, part := range parts {
		endpointType := linodego.ObjectStorageEndpointType(strings.TrimSpace(part))
		if endpointType == "" {
			continue
		}

		switch endpointType {
		case linodego.ObjectStorageEndpointE0,
			linodego.ObjectStorageEndpointE1,
			linodego.ObjectStorageEndpointE2,
			linodego.ObjectStorageEndpointE3:
			preferences = append(preferences, endpointType)
		default:
			return nil, fmt.Errorf("%w: %s", ErrUnknownEndpointType, endpointType)
		}
	}

	if len(preferences) == 0 {
		return nil, nil
	}

	return preferences, nil
}

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
	ErrUnknownEndpointType = errors.New("unknown endpoint type")
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
	KeyBucketEndpointType      = "bucket.endpoint_type"
	KeyBucketAccessIDRaw       = "bucket.access.id_raw"
	KeyBucketAccessID          = "bucket.access.id"
	KeyBucketAccessName        = "bucket.access.name"
	KeyBucketAccessAuth        = "bucket.access.auth"
	KeyBucketAccessPermissions = "bucket.access.permissions"
)
