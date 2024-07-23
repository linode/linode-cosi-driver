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

package metrics

import (
	"context"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

const meterName = "github.com/linode/linode-cosi-driver/pkg/observability/metrics"

func Setup(ctx context.Context, resource *resource.Resource) (_ func(context.Context) error, err error) {
	reader, err := autoexport.NewMetricReader(ctx)
	if err != nil {
		return nil, err
	}

	options := []sdkmetric.Option{
		sdkmetric.WithReader(reader),
	}
	if resource != nil {
		options = append(options, sdkmetric.WithResource(resource))
	}

	mp := sdkmetric.NewMeterProvider(options...)
	otel.SetMeterProvider(mp)

	return mp.Shutdown, nil
}

func Meter() metric.Meter {
	return otel.Meter(meterName)
}
