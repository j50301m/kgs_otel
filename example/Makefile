# Makefile for compiling protocol buffers

# Variables
PROTO_DIR := api
GO_OUT_DIR := ./api

# Compile protocol buffers
proto_build:
	protoc -I $(PROTO_DIR) \
		--go_out=paths=source_relative:$(GO_OUT_DIR) \
		--go-grpc_out=paths=source_relative:$(GO_OUT_DIR) \
		$(PROTO_DIR)/*.proto


# Clean generated files (optional)
clean:
	rm -f $(GO_OUT_DIR)/*.pb.go

.PHONY: proto_build clean