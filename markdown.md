# Kgs Otel

這是一個基於opentelemetry-go的集成工具 主要實現了gin與grpc中tracing的功能
讓開發中可以調用簡單的方法達到監控服務與日誌的功能

可以參考使用 [`example`]來實現功能

# Usage

下面是一個簡單的例子展示如何初始化telemetry與gin server如何貼添加中間件
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

        // Log some message
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