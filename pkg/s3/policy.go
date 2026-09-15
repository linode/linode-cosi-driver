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
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"
)

type PolicyTemplateParams struct {
	BucketName string
}

type policyDocument struct {
	Statement []policyStatement `json:"Statement"`
}

type policyStatement struct {
	Effect    string          `json:"Effect"`
	Principal json.RawMessage `json:"Principal"`
	Action    json.RawMessage `json:"Action"`
	Resource  json.RawMessage `json:"Resource"`
}

func ApplyTemplate(policy string, params PolicyTemplateParams) (string, error) {
	tpl, err := template.New("").Parse(policy)
	if err != nil {
		return "", fmt.Errorf("failed to parse policy: %w", err)
	}

	buf := new(bytes.Buffer)
	if err := tpl.Execute(buf, params); err != nil {
		return "", fmt.Errorf("failed to execute policy template: %w", err)
	}

	return buf.String(), nil
}

// ValidatePolicy validates the required shape of a bucket policy.
func ValidatePolicy(policy string) error {
	document, err := decodePolicy(policy)
	if err != nil {
		return err
	}
	if len(document.Statement) == 0 {
		return fmt.Errorf("policy must contain at least one statement")
	}

	for index, statement := range document.Statement {
		if statement.Effect != "Allow" && statement.Effect != "Deny" {
			return fmt.Errorf("statement %d must have an Allow or Deny effect", index)
		}
		if !validPolicyValue(statement.Principal) {
			return fmt.Errorf("statement %d must have a non-empty principal", index)
		}
		if !validPolicyValue(statement.Action) {
			return fmt.Errorf("statement %d must have at least one action", index)
		}
		if !validPolicyValue(statement.Resource) {
			return fmt.Errorf("statement %d must have at least one resource", index)
		}
	}
	return nil
}

// IsPublicPolicy reports whether a bucket policy contains an Allow statement
// with a wildcard principal.
func IsPublicPolicy(policy string) bool {
	document, err := decodePolicy(policy)
	if err != nil {
		return false
	}
	for _, statement := range document.Statement {
		if statement.Effect == "Allow" && bytes.Equal(statement.Principal, []byte(`"*"`)) {
			return true
		}
	}
	return false
}

func decodePolicy(policy string) (policyDocument, error) {
	var document policyDocument
	if err := json.Unmarshal([]byte(policy), &document); err != nil {
		return policyDocument{}, fmt.Errorf("invalid policy JSON: %w", err)
	}
	return document, nil
}

func validPolicyValue(raw json.RawMessage) bool {
	var value any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return false
	}
	return validPolicyValueDecoded(value)
}

func validPolicyValueDecoded(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed != ""
	case []any:
		if len(typed) == 0 {
			return false
		}
		for _, item := range typed {
			if !validPolicyValueDecoded(item) {
				return false
			}
		}
		return true
	case map[string]any:
		if len(typed) == 0 {
			return false
		}
		for _, item := range typed {
			if !validPolicyValueDecoded(item) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
