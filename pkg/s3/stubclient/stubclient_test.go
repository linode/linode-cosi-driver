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

package stubclient

import (
	"testing"
)

func TestValidatePolicy(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		policy   string
		expected bool
	}{
		"Empty Policy": {
			expected: true,
		},
		"Valid Policy": {
			policy: `{
				"Id": "Policy1741694611374",
				"Version": "2012-10-17",
				"Statement": [
					{
						"Sid": "Stmt1741694555297",
						"Action": "s3:*",
						"Effect": "Allow",
						"Resource": "*",
						"Principal": "*"
					}
				]
			}`,
			expected: true,
		},
		"Multiple Statements": {
			policy: `{
				"Id": "PolicyMultiStmt",
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Action": "s3:ListBucket",
						"Resource": "arn:aws:s3:::example-bucket",
						"Principal": "*"
					},
					{
						"Effect": "Deny",
						"Action": "s3:DeleteObject",
						"Resource": "arn:aws:s3:::example-bucket/*",
						"Principal": "*"
					}
				]
			}`,
			expected: true,
		},
		"Multiple Resources": {
			policy: `{
				"Id": "PolicyMultiRes",
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Action": "s3:GetObject",
						"Resource": [
							"arn:aws:s3:::example-bucket-1/*",
							"arn:aws:s3:::example-bucket-2/*"
						],
						"Principal": "*"
					}
				]
			}`,
			expected: true,
		},
		"Multiple Principals": {
			policy: `{
				"Id": "PolicyMultiPrin",
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Action": "s3:PutObject",
						"Resource": "arn:aws:s3:::example-bucket/*",
						"Principal": {
							"AWS": [
								"arn:aws:iam::111122223333:user/Alice",
								"arn:aws:iam::444455556666:user/Bob"
							]
						}
					}
				]
			}`,
			expected: true,
		},
		"Missing Version": {
			policy: `{
				"Statement": [
					{
						"Sid": "Stmt1741694555297",
						"Action": "s3:*",
						"Effect": "Allow",
						"Resource": "*",
						"Principal": "*"
					}
				]
			}`,
		},
		"Empty Statement": {
			policy: `{
				"Id": "Policy1741694611374",
				"Version": "2012-10-17",
				"Statement": []
			}`,
		},
		"Invalid JSON": {
			policy: `{ "Version": "2012-10-17", "Statement": [ { "Effect": "Allow", "Action": "s3:*", "Resource": "*", "Principal": * } ] }`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actual := validatePolicy(tc.policy)
			if actual != tc.expected {
				t.Errorf("expected: %v, but got: %v", tc.expected, actual)
			}
		})
	}
}
