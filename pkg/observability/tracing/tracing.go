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

package tracing

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	exp, err := autoexport.NewSpanExporter(ctx)
	if err != nil {
		return nil, err
	}

	return registerTraceExporter(resource, exp)
}

func registerTraceExporter(res *resource.Resource, exporter sdktrace.SpanExporter) (func(context.Context) error, error) {
	options := []sdktrace.TracerProviderOption{
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(defaultSamplingRatio)),
	}
	if res != nil {
		options = append(options, sdktrace.WithResource(res))
	}

	tp := sdktrace.NewTracerProvider(options...)
	otel.SetTracerProvider(tp)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})

	if os.Getenv("OTEL_METRICS_EXPORTER") == "prometheus" {
		lis, err := net.Listen("tcp", ":8080") // #nosec G102
		if err != nil {
			return nil, fmt.Errorf("failed to start listener for prometheus metrics server: %w", err)
		}

		srv := new(http.Server)
		mux := new(http.ServeMux)
		mux.Handle("/metrics", promhttp.Handler())
		srv.Handler = mux

		if err := srv.Serve(lis); err != nil {
			return nil, fmt.Errorf("failed to start server for prometheus metrics server: %w", err)
		}

		return func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer cancel()

			var err error

			if srvErr := srv.Shutdown(ctx); srvErr != nil {
				err = errors.Join(err, srvErr)
			}

			if lisErr := lis.Close(); lisErr != nil {
				err = errors.Join(err, lisErr)
			}

			if expErr := tp.Shutdown(ctx); expErr != nil {
				err = errors.Join(err, expErr)
			}

			return err
		}, nil
	}

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
