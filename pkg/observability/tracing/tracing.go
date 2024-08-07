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

package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/contrib/exporters/autoexport"
	"go.opentelemetry.io/otel"
	otelcodes "go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	tracerName           = "github.com/linode/linode-cosi-driver/pkg/observability/tracing"
	defaultSamplingRatio = 1
)

func Setup(ctx context.Context, resource *resource.Resource) (_ func(context.Context) error, err error) {
	exporter, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, err
	}

	options := []sdktrace.TracerProviderOption{
		sdktrace.WithBatcher(exporter),
	}
	if resource != nil {
		options = append(options, sdktrace.WithResource(resource))
	}

	tp := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(tp)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Shutdown will flush any remaining spans and shut down the exporter.
	return tp.Shutdown, nil
}

func Start(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, name)
}

// Error returns an error representing code and error message and records new event on the span.
// If code is OK, returns nil.
func Error(span trace.Span, code grpccodes.Code, err error, events ...string) error {
	if span != nil {
		for _, event := range events {
			span.AddEvent(event)
		}
	}

	if err != nil && span != nil {
		span.RecordError(err)

		if code != grpccodes.OK {
			span.SetStatus(otelcodes.Error, err.Error())
		}
	}

	return status.Error(code, fmt.Sprintf("%v", err))
}
