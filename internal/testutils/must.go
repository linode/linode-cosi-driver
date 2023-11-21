// Copyright 2023 Linode, LLC
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

// Do takes a value of any type and an error. If the error is not nil, it panics with the given error.
// Otherwise, it returns the value. This function is useful for handling errors that are not expected to occur
// and can be safely ignored or handled with a panic.
//
// Example:
//
//	result := must.Do(someFunction())
//	// If someFunction() returns an error, the program will panic with that error.
//	// Otherwise, the result will be assigned the value returned by someFunction().
func Must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}

	return value
}
