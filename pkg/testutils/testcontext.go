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

package testutils

import (
	"context"
	"testing"
	"time"
)

var DefaultTimeout = time.Second * 30

func ContextFromT(ctx context.Context, t *testing.T) (context.Context, context.CancelFunc) {
	return ContextFromTimeout(ctx, t, DefaultTimeout)
}

func ContextFromTimeout(ctx context.Context, t *testing.T, timeout time.Duration) (context.Context, context.CancelFunc) {
	d := time.Now().Add(timeout)
	return ContextFromDeadline(ctx, t, d)
}

func ContextFromDeadline(ctx context.Context, t *testing.T, deadline time.Time) (context.Context, context.CancelFunc) {
	testDeadline, ok := t.Deadline()
	if ok {
		return contextFromTimes(ctx, testDeadline, deadline)
	}

	// this should be unreachable
	return contextFromTimes(ctx, testDeadline, testDeadline)
}

func contextFromTimes(ctx context.Context, t1, t2 time.Time) (context.Context, context.CancelFunc) {
	if t1.Before(t2) {
		return context.WithDeadline(ctx, t1)
	}

	return context.WithDeadline(ctx, t2)
}
