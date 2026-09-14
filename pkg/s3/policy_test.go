// Copyright 2025 Akamai Technologies, Inc.
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

package s3

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestApplyTemplate(t *testing.T) {
	t.Parallel()

	const (
		testPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": "*",
      "Resource": [
        "arn:aws:s3:::{{ .BucketName }}",
        "arn:aws:s3:::{{ .BucketName }}/*"
      ]
    }
  ]
}`

		expectedPolicy = `{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": "*",
      "Resource": [
        "arn:aws:s3:::test-bucket",
        "arn:aws:s3:::test-bucket/*"
      ]
    }
  ]
}`
	)

	for name, tc := range map[string]struct {
		policy        string
		params        PolicyTemplateParams
		expected      string
		expectedError error
	}{
		"basic": {
			policy:   testPolicy,
			params:   PolicyTemplateParams{BucketName: "test-bucket"},
			expected: expectedPolicy,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actual, err := ApplyTemplate(tc.policy, tc.params)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}

			if normalizeJSON(t, actual) != normalizeJSON(t, tc.expected) {
				t.Errorf("expected policy: %v, but got: %v", tc.expected, actual)
			}
		})
	}
}

func TestValidatePolicy(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		policy  string
		public  bool
		wantErr bool
	}{
		"wildcard string": {
			policy: `{"Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::bucket/*"}]}`,
			public: true,
		},
		"deny wildcard": {
			policy: `{"Statement":[{"Effect":"Deny","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::bucket/*"}]}`,
		},
		"named principal": {
			policy: `{"Statement":[{"Effect":"Allow","Principal":{"AWS":"user"},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::bucket/*"]}]}`,
		},
		"invalid JSON": {
			policy:  `{`,
			wantErr: true,
		},
		"missing action": {
			policy:  `{"Statement":[{"Effect":"Allow","Principal":"*","Resource":"arn:aws:s3:::bucket/*"}]}`,
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			public, err := ValidatePolicy(tc.policy)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidatePolicy() error = %v, wantErr %t", err, tc.wantErr)
			}
			if public != tc.public {
				t.Errorf("ValidatePolicy() public = %t, want %t", public, tc.public)
			}
		})
	}
}

func normalizeJSON(t *testing.T, input string) string {
	t.Helper()

	var decoded any
	if err := json.Unmarshal([]byte(input), &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON %q: %v", input, err)
	}

	normalized, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("failed to marshal JSON %q: %v", input, err)
	}

	return string(normalized)
}
