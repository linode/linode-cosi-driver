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

package envflag_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/linode/linode-cosi-driver/pkg/envflag"
)

//nolint:paralleltest
func TestString(t *testing.T) {
	const (
		DefaultValue = "Default"
		Key          = "KEY"
		Value        = "Value"
	)

	for _, tc := range []struct {
		name          string // required
		key           string
		value         string
		defaultValue  string
		expectedValue string
	}{
		{
			name: "simple",
		},
		{
			name:          "with default value",
			defaultValue:  DefaultValue,
			expectedValue: DefaultValue,
		},
		{
			name:          "with actual value",
			key:           Key,
			value:         Value,
			defaultValue:  DefaultValue,
			expectedValue: Value,
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			if tc.key != "" {
				tc.key = fmt.Sprintf("TEST_%d_%s", rand.Intn(256), tc.key) // #nosec G404

				t.Setenv(tc.key, tc.value)
			}

			actual := envflag.String(tc.key, tc.defaultValue)
			if actual != tc.expectedValue {
				t.Errorf("expected: %s, got: %s", tc.expectedValue, actual)
			}
		})
	}
}
