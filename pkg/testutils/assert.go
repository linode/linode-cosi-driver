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

package testutils

import (
	"os"
	"testing"
)

func AssertNotPanics(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("expected no panic, but got one: %v", r)
		}
	}()
	f()
}

func AssertPanics(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic, but got none")
		}
	}()
	f()
}

func AssertDirExists(t *testing.T, dir string) {
	if stat, err := os.Stat(dir); os.IsNotExist(err) || !stat.IsDir() {
		t.Errorf("expected directory %s to exist, but it doesn't", dir)
	}
}
