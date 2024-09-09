package kgsotel

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func InitTelemetry(
	ctx context.Context, serviceName string, otelUrl string) (
	shutdown func(context.Context) error, err error) {

	var shutdownFuncs []func(context.Context) error

	// Shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	finalShutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// When the application is shuting down, we want to send all the remaining
	// If an error occurs during the initialization phase, only need to execute `shutdown｀
	sendAllBeforeShutdown := func(ctx context.Context) error {
		// Send all span before shutdown
		if sdkTP, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); ok {
			sdkTP.ForceFlush(ctx)
		}

		// Send all metrics before shutdown
		if sdkMP, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider); ok {
			sdkMP.ForceFlush(ctx)
		}

		// Send all logs before shutdown
		if sdkLog, ok := global.GetLoggerProvider().(*sdklog.LoggerProvider); ok {
			sdkLog.ForceFlush(ctx)
		}
		return finalShutdown(ctx)
	}

	// HandleErr calls shutdown for cleanup and makes sure that all errors are returned.
	handleErr := func(inErr error) {
		err = errors.Join(inErr, finalShutdown(ctx))
	}

	// Create a new gRPC client connection
	conn, err := initConn(otelUrl)
	if err != nil {
		handleErr(err)
		return finalShutdown, err
	}

	// Initialize the propagator
	initPropagator()

	// Set up a resource with a service name attribute
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.KeyValue{Key: "service.name", Value: attribute.StringValue(serviceName)},
		),
		resource.WithHost(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		handleErr(err)
		return finalShutdown, err
	}

	// Initialize the trace provider
	shutdownTracer, err := initTracerProvider(ctx, res, conn)
	if err != nil {
		handleErr(err)
		return shutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, shutdownTracer)

	// Initialize the meter provider
	shutdownMeter, err := initMeterProvider(ctx, res, conn)
	if err != nil {
		handleErr(err)
		return finalShutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, shutdownMeter)

	// Initialize the logger provider
	shutdownLogger, err := initLoggerProvider(ctx, res, conn, serviceName)
	if err != nil {
		handleErr(err)
		return finalShutdown, err
	}
	shutdownFuncs = append(shutdownFuncs, shutdownLogger)

	// Initialize the logger
	initLogger(serviceName)

	return sendAllBeforeShutdown, nil
}

// Initializes a gRPC client connection to the OpenTelemetry collector.
func initConn(otelUrl string) (*grpc.ClientConn, error) {
	// Create a new gRPC client connection
	conn, err := grpc.NewClient(otelUrl,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("init conn: %w", err)
	}

	return conn, nil
}

func initPropagator() {
	props := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(props)
}

// Initializes an OTLP exporter, and configures the corresponding tracer provider.
func initTracerProvider(ctx context.Context, res *resource.Resource, conn *grpc.ClientConn) (func(context.Context) error, error) {
	// Set up a trace exporter
	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("init trace exporter: %w", err)
	}

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // We want to see all the spans
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(tracerProvider)

	return traceExporter.Shutdown, nil
}

// Initializes an OTLP exporter, and configures the corresponding meter provider.
func initMeterProvider(ctx context.Context, res *resource.Resource, conn *grpc.ClientConn) (func(context.Context) error, error) {
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("create metrics exporter: %w", err)
	}

	// Create a new meter provider
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	// Register the meter provider with the global meter provider
	otel.SetMeterProvider(meterProvider)

	return meterProvider.Shutdown, nil
}

func initLoggerProvider(ctx context.Context, res *resource.Resource, conn *grpc.ClientConn, serviceName string) (func(context.Context) error, error) {
	// Set up a logger exporter
	loggerExporter, err := otlploggrpc.New(ctx, otlploggrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("init logger exporter: %w", err)
	}

	// Create a log record processor pipeline
	processor := sdklog.NewBatchProcessor(loggerExporter)
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(processor),
	)

	// Register the logger provider with the global logger
	global.SetLoggerProvider(loggerProvider)

	return loggerProvider.Shutdown, nil
}
