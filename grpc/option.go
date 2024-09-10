// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Base on https://github.com/open-telemetry/opentelemetry-go-contrib/blob/instrumentation/github.com/gin-gonic/gin/otelgin/v0.54.0/instrumentation/google.golang.org/grpc/otelgrpc/config.go

package otelgrpc

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/stats"
)

const (
	// ScopeName is the instrumentation scope name.
	ScopeName = "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	// GRPCStatusCodeKey is convention for numeric status code of a gRPC request.
	GRPCStatusCodeKey = attribute.Key("rpc.grpc.status_code")
)

// config is a group of options for this instrumentation.
type config struct {
	Filter            Filter
	InterceptorFilter InterceptorFilter
	Propagators       propagation.TextMapPropagator
	TracerProvider    trace.TracerProvider
	MeterProvider     metric.MeterProvider
	SpanStartOptions  []trace.SpanStartOption
	SpanAttributes    []attribute.KeyValue
	MetricAttributes  []attribute.KeyValue

	tracer trace.Tracer
	meter  metric.Meter

	rpcDuration        metric.Float64Histogram
	rpcRequestSize     metric.Int64Histogram
	rpcResponseSize    metric.Int64Histogram
	rpcRequestsPerRPC  metric.Int64Histogram
	rpcResponsesPerRPC metric.Int64Histogram
}

// Filter is a predicate used to determine whether a given request in
// should be instrumented by the attached RPC tag info.
// A Filter must return true if the request should be instrumented.
type Filter func(*stats.RPCTagInfo) bool

// InterceptorFilter is a predicate used to determine whether a given request in
// interceptor info should be instrumented. A InterceptorFilter must return true if
// the request should be traced.
//
// Deprecated: Use stats handlers instead.
type InterceptorFilter func(*InterceptorInfo) bool

// Option applies an option value for a config.
type Option interface {
	apply(*config)
}

type optionFunc func(*config)

func (o optionFunc) apply(c *config) {
	o(c)
}

// WithFilter returns an Option to use the request filter.
func WithFilter(f Filter) Option {
	return optionFunc(func(cfg *config) {
		if f != nil {
			cfg.Filter = f
		}
	})
}

// WithInterceptorFilter returns an Option to use the interceptor filter.
func WithInterceptorFilter(f InterceptorFilter) Option {
	return optionFunc(func(cfg *config) {
		if f != nil {
			cfg.InterceptorFilter = f
		}
	})
}

// WithPropagators returns an Option to use the propagators.
func WithPropagators(propagators propagation.TextMapPropagator) Option {
	return optionFunc(func(cfg *config) {
		if propagators != nil {
			cfg.Propagators = propagators
		}
	})
}

// WithTracerProvider returns an Option to use the tracer provider.
func WithTracerProvider(provider trace.TracerProvider) Option {
	return optionFunc(func(cfg *config) {
		if provider != nil {
			cfg.TracerProvider = provider
		}
	})
}

// WithMeterProvider returns an Option to use the meter provider.
func WithMeterProvider(provider metric.MeterProvider) Option {
	return optionFunc(func(cfg *config) {
		if provider != nil {
			cfg.MeterProvider = provider
		}
	})
}

// WithSpanOptions returns an Option to use the span start options.
func WithSpanOptions(opts ...trace.SpanStartOption) Option {
	return optionFunc(func(cfg *config) {
		cfg.SpanStartOptions = opts
	})
}

// WithSpanAttributes returns an Option to use the span attributes.
func WithSpanAttributes(attrs ...attribute.KeyValue) Option {
	return optionFunc(func(cfg *config) {
		cfg.SpanAttributes = attrs
	})
}

// WithMetricAttributes returns an Option to use the metric attributes.
func WithMetricAttributes(attrs ...attribute.KeyValue) Option {
	return optionFunc(func(cfg *config) {
		cfg.MetricAttributes = attrs
	})
}

// newConfig creates a new config with the given role and options.
func newConfig(role Role, opts ...Option) *config {
	cfg := &config{}
	for _, opt := range opts {
		opt.apply(cfg)
	}
	if cfg.TracerProvider == nil {
		cfg.TracerProvider = otel.GetTracerProvider()
	}
	if cfg.MeterProvider == nil {
		cfg.MeterProvider = otel.GetMeterProvider()
	}
	if cfg.Propagators == nil {
		cfg.Propagators = otel.GetTextMapPropagator()
	}

	// Set the tracer and meter for the service.
	cfg.tracer = cfg.TracerProvider.Tracer(ScopeName)

	cfg.meter = cfg.MeterProvider.Meter(
		ScopeName,
		metric.WithSchemaURL(semconv.SchemaURL),
	)

	var err error

	// Measure the duration of the incoming RPCs.
	cfg.rpcDuration, err = cfg.meter.Float64Histogram("rpc."+role.String()+".duration",
		metric.WithDescription("Measures the duration of inbound RPC."),
		metric.WithUnit("ms"))
	if err != nil {
		otel.Handle(err)
		if cfg.rpcDuration == nil {
			cfg.rpcDuration = noop.Float64Histogram{}
		}
	}

	// Measure the size of the request and response bodies.
	cfg.rpcRequestSize, err = cfg.meter.Int64Histogram("rpc."+role.String()+".request.size",
		metric.WithDescription("Measures size of RPC request messages (uncompressed)."),
		metric.WithUnit("By"))
	if err != nil {
		otel.Handle(err)
		if cfg.rpcRequestSize == nil {
			cfg.rpcRequestSize = noop.Int64Histogram{}
		}
	}

	// Measure the size of the request and response bodies.
	cfg.rpcResponseSize, err = cfg.meter.Int64Histogram("rpc."+role.String()+".response.size",
		metric.WithDescription("Measures size of RPC response messages (uncompressed)."),
		metric.WithUnit("By"))
	if err != nil {
		otel.Handle(err)
		if cfg.rpcResponseSize == nil {
			cfg.rpcResponseSize = noop.Int64Histogram{}
		}
	}

	// Measure the number of messages received per RPC.
	cfg.rpcRequestsPerRPC, err = cfg.meter.Int64Histogram("rpc."+role.String()+".requests_per_rpc",
		metric.WithDescription("Measures the number of messages request per RPC. Should be 1 for all non-streaming RPCs."),
		metric.WithUnit("{count}"))
	if err != nil {
		otel.Handle(err)
		if cfg.rpcRequestsPerRPC == nil {
			cfg.rpcRequestsPerRPC = noop.Int64Histogram{}
		}
	}

	// Measure the number of messages received per RPC.
	cfg.rpcResponsesPerRPC, err = cfg.meter.Int64Histogram("rpc."+role.String()+".responses_per_rpc",
		metric.WithDescription("Measures the number of messages received per RPC. Should be 1 for all non-streaming RPCs."),
		metric.WithUnit("{count}"))
	if err != nil {
		otel.Handle(err)
		if cfg.rpcResponsesPerRPC == nil {
			cfg.rpcResponsesPerRPC = noop.Int64Histogram{}
		}
	}

	return cfg
}
