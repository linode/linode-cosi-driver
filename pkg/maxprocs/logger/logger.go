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

package logger

import (
	"fmt"
	"log/slog"

	"github.com/linode/linode-cosi-driver/pkg/logutils"
	"go.uber.org/automaxprocs/maxprocs"
)

const (
	component = "maxprocs"
)

func Wrap(handler slog.Handler) func(msg string, fields ...any) {
	handler = handler.WithAttrs([]slog.Attr{
		slog.String(logutils.KeyComponentName, component),
		slog.String(logutils.KeyComponentVersion, maxprocs.Version),
	})

	log := slog.New(handler)

	return func(msg string, fields ...any) {
		log.Info(fmt.Sprintf(msg, fields...))
	}
}
