// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Base on https://github.com/open-telemetry/opentelemetry-go-contrib/blob/instrumentation/github.com/gin-gonic/gin/otelgin/v0.54.0/instrumentation/github.com/gin-gonic/gin/otelgin/option.go

package otelgin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type config struct {
	TracerProvider    oteltrace.TracerProvider
	MeterProvider     otelmetric.MeterProvider
	Propagators       propagation.TextMapPropagator
	Filters           []Filter
	GinFilters        []GinFilter
	SpanNameFormatter SpanNameFormatter

	reqDuration otelmetric.Float64Histogram
	reqSize     otelmetric.Int64UpDownCounter
	respSize    otelmetric.Int64UpDownCounter
	activeReqs  otelmetric.Int64UpDownCounter
}

// Adding new Filter parameter (*gin.Context)
// gin.Context has FullPath() method, which returns a matched route full path.
type GinFilter func(*gin.Context) bool

// Filter is a predicate used to determine whether a given http.request should
// be traced. A Filter must return true if the request should be traced.
type Filter func(*http.Request) bool

// SpanNameFormatter is used to set span name by http.request.
type SpanNameFormatter func(r *http.Request) string

// Option specifies instrumentation configuration options.
type Option interface {
	apply(*config)
}

type optionFunc func(*config)

func (o optionFunc) apply(c *config) {
	o(c)
}

// WithPropagators specifies propagators to use for extracting
// information from the HTTP requests. If none are specified, global
// ones will be used.
func WithPropagators(propagators propagation.TextMapPropagator) Option {
	return optionFunc(func(cfg *config) {
		if propagators != nil {
			cfg.Propagators = propagators
		}
	})
}

// WithTracerProvider specifies a tracer provider to use for creating a tracer.
// If none is specified, the global provider is used.
func WithTracerProvider(provider oteltrace.TracerProvider) Option {
	return optionFunc(func(cfg *config) {
		if provider != nil {
			cfg.TracerProvider = provider
		}
	})
}

// WithMeterProvider specifies a meter provider to use for creating a metric.
// If none is specified, the global provider is used.
func WithMeterProvider(provider otelmetric.MeterProvider) Option {
	return optionFunc(func(cfg *config) {
		if provider != nil {
			cfg.MeterProvider = provider
		}
	})
}

// WithFilter adds a filter to the list of filters used by the handler.
// If any filter indicates to exclude a request then the request will not be
// traced. All filters must allow a request to be traced for a Span to be created.
// If no filters are provided then all requests are traced.
// Filters will be invoked for each processed request, it is advised to make them
// simple and fast.
func WithFilter(f ...Filter) Option {
	return optionFunc(func(c *config) {
		c.Filters = append(c.Filters, f...)
	})
}

// WithSpanNameFormatter takes a function that will be called on every
// request and the returned string will become the Span Name.
func WithSpanNameFormatter(f func(r *http.Request) string) Option {
	return optionFunc(func(c *config) {
		c.SpanNameFormatter = f
	})
}

// WithGinFilter adds a gin filter to the list of filters used by the handler.
func WithGinFilter(f ...GinFilter) Option {
	return optionFunc(func(c *config) {
		c.GinFilters = append(c.GinFilters, f...)
	})
}
