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
	"reflect"
	"testing"

	cosi "sigs.k8s.io/container-object-storage-interface-spec"
)

func TestCredentials(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		region      string
		endpoint    string
		label       string
		accessKey   string
		secretKey   string
		expected    map[string]*cosi.CredentialDetails
		shouldPanic bool
	}{
		"valid_credentials": {
			region:    "us-east",
			endpoint:  "us-east-1.linodeobjects.com",
			label:     "test-label",
			accessKey: "TESTACCESSKEY",
			secretKey: "TESTSECRETKEY",
			expected: map[string]*cosi.CredentialDetails{
				S3: {
					Secrets: map[string]string{
						S3Region:                "us-east",
						S3Endpoint:              "https://test-label.us-east-1.linodeobjects.com",
						S3SecretAccessKeyID:     "TESTACCESSKEY",
						S3SecretAccessSecretKey: "TESTSECRETKEY",
					},
				},
			},
		},
		"missing_region": {
			region:      "",
			endpoint:    "us-east-1.linodeobjects.com",
			label:       "test-bucket",
			accessKey:   "TESTACCESSKEY",
			secretKey:   "TESTSECRETKEY",
			shouldPanic: true,
		},
		"missing_endpoint": {
			region:      "us-east",
			endpoint:    "",
			label:       "test-bucket",
			accessKey:   "TESTACCESSKEY",
			secretKey:   "TESTSECRETKEY",
			shouldPanic: true,
		},
		"missing_label": {
			region:      "us-east",
			endpoint:    "us-east-1.linodeobjects.com",
			label:       "",
			accessKey:   "TESTACCESSKEY",
			secretKey:   "TESTSECRETKEY",
			shouldPanic: true,
		},
		"missing_accessKey": {
			region:      "us-east",
			endpoint:    "us-east-1.linodeobjects.com",
			label:       "test-bucket",
			accessKey:   "",
			secretKey:   "TESTSECRETKEY",
			shouldPanic: true,
		},
		"missing_secretKey": {
			region:      "us-east",
			endpoint:    "us-east-1.linodeobjects.com",
			label:       "test-bucket",
			accessKey:   "TESTACCESSKEY",
			secretKey:   "",
			shouldPanic: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			defer func() {
				if r := recover(); r != nil {
					if !tc.shouldPanic {
						t.Fatalf("unexpected panic: %v", r)
					}
				} else if tc.shouldPanic {
					t.Fatalf("expected panic but did not occur")
				}
			}()

			actual := credentials(tc.region, tc.endpoint, tc.label, tc.accessKey, tc.secretKey)
			if !reflect.DeepEqual(actual, tc.expected) {
				t.Fatalf("expected %+v, got %+v", tc.expected, actual)
			}
		})
	}
}
