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

package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	o11y "github.com/linode/linode-cosi-driver/pkg/observability"
	"github.com/linode/linode-cosi-driver/pkg/testutils"
)

func TestRun(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		testName      string // required
		options       []func(*mainOptions)
		expectedError error
	}{
		{
			testName: "simple",
			options: []func(*mainOptions){
				func(*mainOptions) { /* noop */ },
			},
		},
	} {
		tc := tc

		t.Run(tc.testName, func(t *testing.T) {
			t.Parallel()

			noopLog := slog.New(slog.NewTextHandler(
				io.Discard,
				&slog.HandlerOptions{Level: slog.LevelError},
			))

			defaultOpts := mainOptions{
				cosiEndpoint:        "cosi.sock",
				otlpTracesProtocol:  o11y.ProtoGRPC,
				otlpMetricsProtocol: o11y.ProtoGRPC,
			}

			for _, opt := range tc.options {
				opt(&defaultOpts)
			}

			ctx, cancel := testutils.ContextFromTimeout(context.Background(), t, time.Second)
			defer cancel()

			tmp := testutils.MustMkdirTemp()
			defer os.RemoveAll(tmp)

			defaultOpts.cosiEndpoint = "unix://" + tmp + defaultOpts.cosiEndpoint

			err := run(ctx, noopLog, defaultOpts)
			if !errors.Is(err, tc.expectedError) {
				t.Errorf("expected error: %v, but got: %v", tc.expectedError, err)
			}
		})
	}
}
