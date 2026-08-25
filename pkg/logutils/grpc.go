// Copyright 2024 Akamai Technologies, Inc.
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

package logutils

import (
	"context"
	"log/slog"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"

	"github.com/linode/linode-cosi-driver/pkg/version"
)

const (
	componentGrpc = "grpc"
)

type GRPCLogger struct {
	loggerImpl *slog.Logger
}

var _ logging.Logger = (*GRPCLogger)(nil)

func ForGRPC(handler slog.Handler) *GRPCLogger {
	handler = handler.WithAttrs([]slog.Attr{
		slog.String(KeyComponentName, componentGrpc),
		slog.String(KeyComponentVersion, version.Version),
	})

	return &GRPCLogger{
		loggerImpl: slog.New(handler),
	}
}

func (l *GRPCLogger) Log(ctx context.Context, level logging.Level, msg string, fields ...any) {
	l.loggerImpl.Log(ctx, slog.Level(level),
		msg,
		fields...)
}
