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

// ValidatePolicy validates the required shape of a bucket policy and reports
// whether an Allow statement grants access to every principal.
func ValidatePolicy(policy string) (bool, error) {
	var document struct {
		Statement []struct {
			Effect    string          `json:"Effect"`
			Principal json.RawMessage `json:"Principal"`
			Action    json.RawMessage `json:"Action"`
			Resource  json.RawMessage `json:"Resource"`
		} `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(policy), &document); err != nil {
		return false, fmt.Errorf("invalid policy JSON: %w", err)
	}
	if len(document.Statement) == 0 {
		return false, fmt.Errorf("policy must contain at least one statement")
	}

	for index, statement := range document.Statement {
		if statement.Effect != "Allow" && statement.Effect != "Deny" {
			return false, fmt.Errorf("statement %d must have an Allow or Deny effect", index)
		}
		if !validPolicyValue(statement.Principal) {
			return false, fmt.Errorf("statement %d must have a non-empty principal", index)
		}
		if !validPolicyValue(statement.Action) {
			return false, fmt.Errorf("statement %d must have at least one action", index)
		}
		if !validPolicyValue(statement.Resource) {
			return false, fmt.Errorf("statement %d must have at least one resource", index)
		}
		if statement.Effect == "Allow" && bytes.Equal(statement.Principal, []byte(`"*"`)) {
			return true, nil
		}
	}
	return false, nil
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
