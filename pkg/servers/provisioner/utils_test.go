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

func TestBucketIDCleanup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		cleanup     ParamCleanupValue
		wantID      string
		wantCleanup bool
	}{
		{name: "cleanup omitted", wantID: "pl-labkrk-2/rc-example"},
		{
			name:        "cleanup forced",
			cleanup:     ParamCleanupForce,
			wantID:      "pl-labkrk-2/rc-example/force",
			wantCleanup: true,
		},
		{
			name:    "unknown cleanup value",
			cleanup: ParamCleanupValue("invalid"),
			wantID:  "pl-labkrk-2/rc-example",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			id := bucketID("pl-labkrk-2", "rc-example", tt.cleanup)
			if id != tt.wantID {
				t.Fatalf("expected bucket ID %q, got %q", tt.wantID, id)
			}
			region, label, cleanup, err := parseBucketID(id)
			if err != nil {
				t.Fatalf("expected valid bucket ID, got error: %v", err)
			}
			if region != "pl-labkrk-2" || label != "rc-example" {
				t.Fatalf("expected bucket location pl-labkrk-2/rc-example, got %s/%s", region, label)
			}
			if cleanup != tt.wantCleanup {
				t.Fatalf("expected cleanup forced to be %t, got %t", tt.wantCleanup, cleanup)
			}
		})
	}
}

func TestParseBucketIDRejectsMalformedIDs(t *testing.T) {
	t.Parallel()

	for _, id := range []string{"", "region", "/label", "region/", "region/label/", "region/label/unknown", "region/label/force/extra"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			if _, _, _, err := parseBucketID(id); err == nil {
				t.Fatalf("expected bucket ID %q to be rejected", id)
			}
		})
	}
}

const (
	testCredentialsRegion    = "us-east"
	testCredentialsEndpoint  = "us-east-1.linodeobjects.com"
	testCredentialsAccessKey = "TESTACCESSKEY"
	testCredentialsSecretKey = "TESTSECRETKEY"
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
			region:    testCredentialsRegion,
			endpoint:  testCredentialsEndpoint,
			label:     "test-label",
			accessKey: testCredentialsAccessKey,
			secretKey: testCredentialsSecretKey,
			expected: map[string]*cosi.CredentialDetails{
				S3: {
					Secrets: map[string]string{
						S3Region:                testCredentialsRegion,
						S3Endpoint:              testCredentialsEndpoint,
						S3SecretAccessKeyID:     testCredentialsAccessKey,
						S3SecretAccessSecretKey: testCredentialsSecretKey,
					},
				},
			},
		},
		"missing_region": {
			region:      "",
			endpoint:    testCredentialsEndpoint,
			label:       "test-bucket",
			accessKey:   testCredentialsAccessKey,
			secretKey:   testCredentialsSecretKey,
			shouldPanic: true,
		},
		"missing_endpoint": {
			region:      testCredentialsRegion,
			endpoint:    "",
			label:       "test-bucket",
			accessKey:   testCredentialsAccessKey,
			secretKey:   testCredentialsSecretKey,
			shouldPanic: true,
		},
		"missing_label": {
			region:      testCredentialsRegion,
			endpoint:    testCredentialsEndpoint,
			label:       "",
			accessKey:   testCredentialsAccessKey,
			secretKey:   testCredentialsSecretKey,
			shouldPanic: true,
		},
		"missing_accessKey": {
			region:      testCredentialsRegion,
			endpoint:    testCredentialsEndpoint,
			label:       "test-bucket",
			accessKey:   "",
			secretKey:   testCredentialsSecretKey,
			shouldPanic: true,
		},
		"missing_secretKey": {
			region:      testCredentialsRegion,
			endpoint:    testCredentialsEndpoint,
			label:       "test-bucket",
			accessKey:   testCredentialsAccessKey,
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
