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

// Package handlers includes common HandlerFuncs that can be used around the gRPC environment.
package grpc

import (
	"context"
	"log/slog"
	"runtime/debug"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"

	"github.com/linode/linode-cosi-driver/pkg/logutils"
	"github.com/linode/linode-cosi-driver/pkg/version"
)

const (
	component = "panic_recovery"
)

// PanicRecovery returns handler of the panics, that logs the panic and call stack.
// It take optional argument called callbacks, that are functions, e.g. wrapping incrementing the panicMetric.
func PanicRecovery(ctx context.Context, handler slog.Handler, callbacks ...func(context.Context)) recovery.RecoveryHandlerFunc {
	handler = handler.WithAttrs([]slog.Attr{
		slog.String(logutils.KeyComponentName, component),
		slog.String(logutils.KeyComponentVersion, version.Version),
	})

	logger := slog.New(handler)

	return func(pan any) (err error) {
		for _, callback := range callbacks {
			callback(ctx)
		}

		if logger != nil {
			logger.Log(ctx, 0,
				"Recovered from panic",
				"panic", pan,
				"stack", debug.Stack())
		}

		return nil
	}
}
