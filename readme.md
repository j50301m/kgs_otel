# Kgs Otel

This is an integration tool based on opentelemetry-go, primarily implementing tracing functionality in gin and grpc.
It allows developers to use simple methods to achieve service monitoring and logging functions during development.

You can refer to the [`example`] to implement the functionality.

# Usage

Below is a simple example showing how to initialize telemetry and how to add middleware to a gin server:

```go
import (
	"context"
	kgsotel "kgs/otel"
	otelgin "kgs/otel/gin"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

    // Initialize telemetry
	shutdown, err := kgsotel.InitTelemetry(ctx, "service_name", "otel_collector_url")
	if err != nil {
		log.Fatal(err)
	}

	// Graceful shutdown
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()

    // Start a gin server
    r := gin.New()
	r.Use(otelgin.TracingMiddleware(_httpServiceName))

	r.GET("/", func(c *gin.Context) {
        // Start a span under the `/version`
		ctx, span := kgsotel.StartTrace(c.Request.Context())
		defer span.End()

        // Log some messages
		kgsotel.Info(ctx, "Hello world!")
        kgsotel.Warn(ctx,"Oops~")
        kgsotel.Error(ctx,"Oh No!")

		c.JSON(200, gin.H{
			"msg": "Hello world!",
		})
	})

    // ...
}
```