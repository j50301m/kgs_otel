package main

import (
	"context"
	kgsotel "kgs/otel"
	otelgin "kgs/otel/gin"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

var serviceName = "kgsotel-http-example"
var otelUrl = "localhost:43177" // Change this to your otlp collector address

func startHttpServer(ctx context.Context) {
	// Initialize telemetry
	shutdown, err := kgsotel.InitTelemetry(ctx, serviceName, otelUrl)
	if err != nil {
		log.Fatal(err)
	}

	// Graceful shutdown
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	r := gin.New()
	r.Use(otelgin.Tracing(serviceName))
	r.GET("/version", func(c *gin.Context) {
		ctx, span := kgsotel.StartTrace(c.Request.Context())
		defer span.End()

		foo(ctx)

		kgsotel.Info(ctx, "version endpoint called")
		c.JSON(200, gin.H{
			"version": "0.1.0",
		})
	})
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	// Initializing the server in a goroutine so that
	// it won't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Listen for the interrupt signal.
	<-ctx.Done()

	log.Println("Http server shut down gracefully...")
}

// foo is a sample function that calls bar.
func foo(ctx context.Context) {
	ctx, span := kgsotel.StartTrace(ctx)
	defer span.End()

	kgsotel.Warn(ctx, "foo called")

	bar(ctx)
}

// bar is a sample function that logs an error.
func bar(ctx context.Context) {
	ctx, span := kgsotel.StartTrace(ctx)
	defer span.End()

	kgsotel.Error(ctx, "bar called")
}
