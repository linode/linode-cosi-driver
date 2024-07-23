// Copyright 2023-2024 Akamai Technologies, Inc.
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

package testutils_test

import (
	"errors"
	"testing"

	"github.com/linode/linode-cosi-driver/pkg/testutils"
)

func TestDo(t *testing.T) {
	t.Parallel()

	defaultAssertion := func(f func()) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("unexpected panic: %#+v", r)
			}
		}()
		f()
	}

	panicAssertion := func(f func()) {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic, but got none")
			}
		}()
		f()
	}

	// test different inputs
	for _, tc := range []struct {
		name      string
		assertion func(func())
		input     func() (any, error)
	}{
		{
			name:      "panics",
			assertion: panicAssertion,
			input: func() (any, error) {
				return nil, errors.New("expecting panic")
			},
		},
		{
			name: "input string",
			input: func() (any, error) {
				return "test", nil
			},
		},
		{
			name: "input int",
			input: func() (any, error) {
				return 1, nil
			},
		},
		{
			name: "input struct",
			input: func() (any, error) {
				return struct{}{}, nil
			},
		},
	} {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.input == nil {
				t.Log("input empty!")
				t.FailNow()
			}

			if tc.assertion == nil {
				tc.assertion = defaultAssertion
			}

			tc.assertion(func() {
				_ = testutils.Must(tc.input())
			})
		})
	}
}
