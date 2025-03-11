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
