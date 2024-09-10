package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	kgsotel "kgs/otel"
	"kgs/otel/example/api"
	otelgrpc "kgs/otel/grpc"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
)

var (
	_grpcServerName = "kgsotel-grpc-example"
	_grpcHost       = "localhost"
	_grpcPort       = "7777"
	_otelUrl        = "localhost:43177" // Change this to your otlp collector address
)

type server struct {
	api.HelloServiceServer
}

// SayHello implements api.HelloServiceServer.
func (s *server) SayHello(ctx context.Context, in *api.HelloRequest) (*api.HelloResponse, error) {
	// Start a trace
	ctx, span := kgsotel.StartTrace(ctx)
	defer span.End()

	kgsotel.Info(ctx, "SayHello", kgsotel.NewFiled("greeting", in.Greeting))

	// Simulate some work
	s.doSomething(ctx)
	time.Sleep(50 * time.Millisecond)

	return &api.HelloResponse{Reply: "Hello " + in.Greeting}, nil
}

func (s *server) doSomething(ctx context.Context) {
	// Start a trace
	ctx, span := kgsotel.StartTrace(ctx)
	defer span.End()

	// Simulate some work
	time.Sleep(50 * time.Millisecond)
	kgsotel.Info(ctx, "doSomething", kgsotel.NewFiled("key", "value"))
}

func (s *server) SayHelloServerStream(in *api.HelloRequest, out api.HelloService_SayHelloServerStreamServer) error {
	// Start a trace
	ctx, span := kgsotel.StartTrace(context.Background())
	defer span.End()

	// Simulate some streaming work
	for i := 0; i < 5; i++ {
		err := out.Send(&api.HelloResponse{Reply: "Hello " + in.Greeting})
		if err != nil {
			return err
		}

		time.Sleep(time.Duration(i*50) * time.Millisecond)
	}

	s.doSomething(ctx)

	return nil
}

func (s *server) SayHelloClientStream(stream api.HelloService_SayHelloClientStreamServer) error {
	// Start a trace
	ctx, span := kgsotel.StartTrace(context.Background())
	defer span.End()
	i := 0

	for {
		in, err := stream.Recv()

		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			kgsotel.Error(ctx, "SayHelloClientStream", kgsotel.NewFiled("error", err))
			return err
		}

		kgsotel.Info(ctx, "SayHelloClientStream", kgsotel.NewFiled("greeting", in.Greeting))
		i++
	}

	time.Sleep(50 * time.Millisecond)

	return stream.SendAndClose(&api.HelloResponse{Reply: fmt.Sprintf("Hello (%v times)", i)})
}

func (s *server) SayHelloBidiStream(stream api.HelloService_SayHelloBidiStreamServer) error {
	// Start a trace
	ctx, span := kgsotel.StartTrace(context.Background())
	defer span.End()

	for {
		in, err := stream.Recv()

		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			kgsotel.Error(ctx, "SayHelloBidiStream", kgsotel.NewFiled("error", err))
			return err
		}

		time.Sleep(50 * time.Millisecond)

		kgsotel.Info(ctx, "SayHelloBidiStream", kgsotel.NewFiled("greeting", in.Greeting))
		err = stream.Send(&api.HelloResponse{Reply: "Hello " + in.Greeting})
		if err != nil {
			kgsotel.Error(ctx, "SayHelloBidiStream", kgsotel.NewFiled("error", err))
			return err
		}
	}

	return nil
}

func startGrpcServer(ctx context.Context) {
	shutdown, err := kgsotel.InitTelemetry(ctx, _grpcServerName, _otelUrl)
	if err != nil {
		log.Fatal(err)
	}

	// Graceful shutdown
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%s", _grpcHost, _grpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.TracingMiddleware(otelgrpc.RoleServer)),
	)

	go func() {
		api.RegisterHelloServiceServer(s, &server{})
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	<-ctx.Done()

	log.Println("gRPC server shut down gracefully...")
}
