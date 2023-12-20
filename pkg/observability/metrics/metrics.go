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

package metrics

import (
	"context"
	"fmt"

	o11y "github.com/linode/linode-cosi-driver/pkg/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

func Setup(ctx context.Context, resource *resource.Resource, protocol string) (_ func(context.Context) error, err error) {
	var exp sdkmetric.Exporter

	switch protocol {
	case o11y.ProtoGRPC:
		exp, err = otlpmetricgrpc.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("unable to create new OTLP Metric GRPC Exporter: %w", err)
		}

	case o11y.ProtoHTTPJSON, o11y.ProtoHTTPProtobuf:
		exp, err = otlpmetrichttp.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("unable to create new OTLP Metric GRPC Exporter: %w", err)
		}
	}

	return registerMetricsExporter(resource, exp)
}

func registerMetricsExporter(res *resource.Resource, exporter sdkmetric.Exporter) (func(context.Context) error, error) {
	// This reader is used as a stand-in for a reader that will actually export
	// data. See exporters in the go.opentelemetry.io/otel/exporters package
	// for more information.
	reader := sdkmetric.NewPeriodicReader(exporter)

	options := []sdkmetric.Option{
		sdkmetric.WithReader(reader),
	}
	if res != nil {
		options = append(options, sdkmetric.WithResource(res))
	}

	mp := sdkmetric.NewMeterProvider(options...)
	otel.SetMeterProvider(mp)

	return mp.Shutdown, nil
}
