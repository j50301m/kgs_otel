package kgsotel

import (
	"context"
	"fmt"
	"runtime"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type Field struct {
	Key   string
	Value interface{}
}

func NewFiled(key string, value interface{}) Field {
	return Field{
		Key:   key,
		Value: value,
	}
}

func Info(ctx context.Context, message string, fields ...Field) {
	span, zapFields := setSpanAttrsAndZapFields(ctx, fields...)
	span.AddEvent(message)
	zap.L().Info(message, zapFields...)
}

func Warn(ctx context.Context, message string, fields ...Field) {
	span, zapFields := setSpanAttrsAndZapFields(ctx, fields...)
	span.AddEvent(message)
	span.SetStatus(codes.Error, message)
	zap.L().Warn(message, zapFields...)
}

func Error(ctx context.Context, message string, fields ...Field) {
	span, zapFields := setSpanAttrsAndZapFields(ctx, fields...)
	span.AddEvent(message)
	span.SetStatus(codes.Error, message)
	zap.L().Error(message, zapFields...)
}

func StartTrace(ctx context.Context) (context.Context, trace.Span) {
	tracer := otel.Tracer("") // The name of the tracer is not important
	caller, funcName := getCaller(2)
	ctx, span := tracer.Start(ctx, funcName)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()

	attributes := []attribute.KeyValue{
		attribute.String("traceID", traceID),
		attribute.String("spanID", spanID),
		attribute.String("caller", caller),
		attribute.String("funcName", funcName),
	}

	span.SetAttributes(attributes...)

	return ctx, span
}

func setSpanAttrsAndZapFields(ctx context.Context, fields ...Field) (span trace.Span, zapFields []zap.Field) {
	span = trace.SpanFromContext(ctx)
	traceID := span.SpanContext().TraceID().String()
	spanID := span.SpanContext().SpanID().String()
	caller, funcName := getCaller(3)

	// Create attributes for span and zap logger
	attributes := []attribute.KeyValue{
		attribute.String("traceID", traceID),
		attribute.String("spanID", spanID),
		attribute.String("caller", caller),
		attribute.String("funcName", funcName),
	}

	zapFields = []zap.Field{
		zap.String("traceID", traceID),
		zap.String("spanID", spanID),
		zap.String("caller", caller),
		zap.String("funcName", funcName),
	}

	for _, field := range fields {
		attributes = append(attributes, attribute.String(field.Key, fmt.Sprintf("%v", field.Value)))
		zapFields = append(zapFields, zap.Any(field.Key, field.Value))
	}
	span.SetAttributes(attributes...)

	return span, zapFields

}

func getCaller(skip int) (caller string, funcName string) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "unknown", "unknown"
	}
	fn := runtime.FuncForPC(pc)
	return fmt.Sprintf("%s:%d", file, line), fn.Name()
}
