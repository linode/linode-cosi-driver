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
	"fmt"
	"strings"

	cosi "sigs.k8s.io/container-object-storage-interface-spec"
)

const bucketIDParts = 3

func parseBucketID(id string) (region, label string, cleanup bool, err error) {
	parts := strings.SplitN(id, "/", bucketIDParts)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false, fmt.Errorf("invalid bucket ID %q", id)
	}

	if len(parts) == bucketIDParts {
		cleanupValue := ParamCleanupValue(parts[2])
		if !cleanupValue.Force() {
			return "", "", false, fmt.Errorf("invalid bucket cleanup policy %q", parts[2])
		}
		cleanup = true
	}

	return parts[0], parts[1], cleanup, nil
}

func bucketID(region, label string, cleanup ParamCleanupValue) string {
	id := region + "/" + label
	if cleanup.Force() {
		return id + "/" + string(ParamCleanupForce)
	}

	return id
}

func bucketInfo(region string) *cosi.Protocol {
	return &cosi.Protocol{
		Type: &cosi.Protocol_S3{
			S3: &cosi.S3{
				Region:           region,
				SignatureVersion: cosi.S3SignatureVersion_S3V4,
			},
		},
	}
}

func credentials(region, endpoint, label, accessKey, secretKey string) map[string]*cosi.CredentialDetails {
	if region == "" {
		panic("region cannot be empty")
	}

	if endpoint == "" {
		panic("endpoint cannot be empty")
	}

	if label == "" {
		panic("label cannot be empty")
	}

	if accessKey == "" || secretKey == "" {
		panic("accessKey or secretKey cannot be empty")
	}

	return map[string]*cosi.CredentialDetails{
		S3: {
			Secrets: map[string]string{
				S3Region:                region,
				S3Endpoint:              endpoint,
				S3SecretAccessKeyID:     accessKey,
				S3SecretAccessSecretKey: secretKey,
			},
		},
	}
}
