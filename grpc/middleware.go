// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Base on https://github.com/open-telemetry/opentelemetry-go-contrib/blob/instrumentation/github.com/gin-gonic/gin/otelgin/v0.54.0/instrumentation/google.golang.org/grpc/otelgrpc/stats_handler.go

package otelgrpc

import (
	"context"
	"kgs/otel/internal"
	"kgs/otel/internal/semconvutil"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
	"go.opentelemetry.io/otel/trace"
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

// gRPCContextKey is a O size type to use as key for context values.
type gRPCContextKey struct{}

type gRPCContext struct {
	messagesReceived int64
	messagesSent     int64
	metricAttrs      []attribute.KeyValue
	record           bool
}

type middleware struct {
	config *config
	role   Role
}

func TracingMiddleware(role Role, opts ...Option) stats.Handler {
	m := &middleware{
		config: newConfig(role, opts...),
		role:   role,
	}

	return m
}

// TagConn can attach some information to the given context.
func (m *middleware) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	return ctx
}

// HandleConn processes the Conn stats.
func (m *middleware) HandleConn(ctx context.Context, info stats.ConnStats) {
}

// TagRPC can attach some information to the given context.
func (m *middleware) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	ctx = extract(ctx, m.config.Propagators)

	var spanKind trace.SpanKind
	if m.role.isServer() {
		spanKind = trace.SpanKindServer
	} else {
		spanKind = trace.SpanKindClient
	}

	name, attrs := internal.ParseFullMethod(info.FullMethodName)
	attrs = append(attrs, semconv.RPCSystemGRPC)
	ctx, _ = m.config.tracer.Start(
		trace.ContextWithRemoteSpanContext(ctx, trace.SpanContextFromContext(ctx)),
		name,
		trace.WithSpanKind(spanKind),
		trace.WithAttributes(append(attrs, m.config.SpanAttributes...)...),
	)

	gctx := gRPCContext{
		metricAttrs: append(attrs, m.config.MetricAttributes...),
		record:      true,
	}
	if m.config.Filter != nil {
		gctx.record = m.config.Filter(info)
	}

	// If role is server then return context with gRPCContextKey.
	if m.role.isServer() {
		return context.WithValue(ctx, gRPCContextKey{}, &gctx)
	}

	// If role is client then inject the current context
	return inject(context.WithValue(ctx, gRPCContextKey{}, &gctx), m.config.Propagators)
}

// HandleRPC processes the RPC stats.
func (m *middleware) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	span := trace.SpanFromContext(ctx)
	var metricAttrs []attribute.KeyValue
	// var messageId int64

	gctx, _ := ctx.Value(gRPCContextKey{}).(*gRPCContext)
	if gctx != nil {
		if !gctx.record {
			return
		}
		metricAttrs = make([]attribute.KeyValue, 0, len(gctx.metricAttrs)+1)
		metricAttrs = append(metricAttrs, gctx.metricAttrs...)
	}

	switch rs := rs.(type) {
	case *stats.Begin:
	case *stats.InPayload:
		if gctx != nil {
			m.config.rpcRequestSize.Record(ctx, int64(rs.Length), metric.WithAttributeSet(attribute.NewSet(metricAttrs...)))
		}

	case *stats.OutPayload:
		if gctx != nil {
			// messageId = atomic.AddInt64(&gctx.messagesSent, 1)
			m.config.rpcResponseSize.Record(ctx, int64(rs.Length), metric.WithAttributeSet(attribute.NewSet(metricAttrs...)))
		}

	case *stats.OutTrailer:
	case *stats.OutHeader:
		if p, ok := peer.FromContext(ctx); ok {
			span.SetAttributes(semconvutil.NetTransport(p.Addr.Network()))
		}
	case *stats.End:
		var rpcStatusAttr attribute.KeyValue

		if rs.Error != nil {
			s, _ := status.FromError(rs.Error)
			if m.role.isServer() {
				statusCode, msg := serverStatus(s)
				span.SetStatus(statusCode, msg)
			} else {
				span.SetStatus(codes.Error, s.Message())
			}
			rpcStatusAttr = semconv.RPCGRPCStatusCodeKey.Int(int(s.Code()))
		} else {
			rpcStatusAttr = semconv.RPCGRPCStatusCodeKey.Int(int(grpcCodes.OK))
		}
		span.SetAttributes(rpcStatusAttr)
		span.End()

		metricAttrs = append(metricAttrs, rpcStatusAttr)
		// Allocate vararg slice once.
		recordOpts := []metric.RecordOption{metric.WithAttributeSet(attribute.NewSet(metricAttrs...))}

		// Use floating point division here for higher precision (instead of Millisecond method).
		// Measure right before calling Record() to capture as much elapsed time as possible.
		elapsedTime := float64(rs.EndTime.Sub(rs.BeginTime)) / float64(time.Millisecond)

		m.config.rpcDuration.Record(ctx, elapsedTime, recordOpts...)
		if gctx != nil {
			m.config.rpcRequestsPerRPC.Record(ctx, atomic.LoadInt64(&gctx.messagesReceived), recordOpts...)
			m.config.rpcResponsesPerRPC.Record(ctx, atomic.LoadInt64(&gctx.messagesSent), recordOpts...)
		}
	default:
		return
	}

}

// serverStatus returns a span status code and message for a given gRPC
// status code. It maps specific gRPC status codes to a corresponding span
// status code and message. This function is intended for use on the server
// side of a gRPC connection.
//
// If the gRPC status code is Unknown, DeadlineExceeded, Unimplemented,
// Internal, Unavailable, or DataLoss, it returns a span status code of Error
// and the message from the gRPC status. Otherwise, it returns a span status
// code of Unset and an empty message.
func serverStatus(grpcStatus *status.Status) (codes.Code, string) {
	switch grpcStatus.Code() {
	case grpcCodes.Unknown,
		grpcCodes.DeadlineExceeded,
		grpcCodes.Unimplemented,
		grpcCodes.Internal,
		grpcCodes.Unavailable,
		grpcCodes.DataLoss:
		return codes.Error, grpcStatus.Message()
	default:
		return codes.Unset, ""
	}
}
