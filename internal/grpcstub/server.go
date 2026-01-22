// Package grpcstub provides functionality to create and manage a gRPC server
// that can load proto definitions and corresponding stub definitions.
package grpcstub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	grpcreflection "google.golang.org/grpc/reflection"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// Repository defines the interface for storing and retrieving gRPC stubs.
type Repository interface {
	Add(stub ProtoStub)
	Get(service string, method string, in json.RawMessage) (Output, bool)
}

// GRPCService represents a gRPC service that can handle requests based on loaded stubs.
type GRPCService struct {
	stubs      Repository
	sdMap      map[string]protoreflect.ServiceDescriptor
	grpcServer *grpc.Server
	files      *protoregistry.Files
	types      *protoregistry.Types
}

// NewServer creates a new gRPC server, loads proto definitions from the
// specified protoDir, and loads stub definitions from the specified protoStubDir.
func NewServer(protoDir string, protoStubDir string) (*grpc.Server, error) {
	server := grpc.NewServer()
	if err := registerServices(server, protoDir, protoStubDir, NewStorage()); err != nil {
		return nil, fmt.Errorf("register services: %w", err)
	}

	return server, nil
}

// registerServices loads proto files from the specified protoDir, registers them with the provided
// gRPC server, and loads stub definitions from the specified stubDir into the provided Repository.
func registerServices(srv *grpc.Server, protoDir string, stubDir string, r Repository) error {
	s := &GRPCService{
		stubs:      r,
		sdMap:      map[string]protoreflect.ServiceDescriptor{},
		grpcServer: srv,
		files:      &protoregistry.Files{},
		types:      &protoregistry.Types{},
	}

	if err := s.registerTypes(protoDir); err != nil {
		return fmt.Errorf("load protos from %v: %w", protoDir, err)
	}

	s.registerServices()
	s.registerReflection()

	if err := s.loadStubs(stubDir); err != nil {
		return fmt.Errorf("load stubs from %v: %w", stubDir, err)
	}

	return nil
}

func (s *GRPCService) registerReflection() {
	opts := grpcreflection.ServerOptions{
		Services:           s.grpcServer,
		DescriptorResolver: s.files,
		ExtensionResolver:  s.types,
	}
	reflectionv1.RegisterServerReflectionServer(s.grpcServer, grpcreflection.NewServerV1(opts))
}

// Handler handles unary gRPC calls by matching them against loaded stubs and returning
// the corresponding responses.
func (s *GRPCService) Handler(_ any, ctx context.Context, deccode func(any) error, _ grpc.UnaryServerInterceptor) (interface{}, error) { //nolint:revive
	stream := grpc.ServerTransportStreamFromContext(ctx)
	serviceName, methodName, err := parseGRPCMethod(stream.Method())
	if err != nil {
		slog.ErrorContext(ctx, "Invalid method format", slog.String("method", stream.Method()))
		return nil, status.Error(codes.InvalidArgument, "Invalid method format")
	}

	slog.InfoContext(ctx, "Received gRPC call", slog.String("service", serviceName), slog.String("method", methodName))

	service, ok := s.sdMap[serviceName]
	if !ok {
		slog.ErrorContext(ctx, "No stub found", slog.String("service", serviceName))
		return nil, status.Error(codes.Unimplemented, "Service "+serviceName+" not found")
	}

	method := service.Methods().ByName(protoreflect.Name(methodName))
	if method == nil {
		return nil, status.Error(codes.Unimplemented, "Method "+methodName+" not found")
	}
	input := dynamicpb.NewMessage(method.Input())

	if err := deccode(input); err != nil {
		slog.ErrorContext(ctx, "Failed to decode input message", slog.String("error", err.Error()))
		return nil, status.Error(codes.InvalidArgument, "Failed to decode input message")
	}

	jsonInput, err := protojson.Marshal(input)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to marshall input", slog.String("error", err.Error()))
		return nil, status.Error(codes.InvalidArgument, "Failed to marshall input")
	}

	resp, ok := s.stubs.Get(serviceName, methodName, jsonInput)
	if !ok {
		slog.ErrorContext(ctx, "No stub configured", slog.String("service", serviceName), slog.String("method", methodName))
		return nil, status.Error(codes.NotFound, "No stub configured")
	}

	if resp.Data != nil {
		output := dynamicpb.NewMessage(method.Output())

		err = protojson.Unmarshal(resp.Data, output)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to unmarshal response", slog.String("error", err.Error()))
			return nil, status.Error(codes.Internal, "Failed to unmarshal response")
		}

		return output, nil
	}

	if resp.Code != nil {
		return nil, status.Error(*resp.Code, resp.Error)
	}

	return nil, status.Error(codes.Unimplemented, resp.Error)
}

// ServerStreamHandler handles server-side streaming gRPC calls by matching them against
// loaded stubs and returning the corresponding stream of responses.
func (s *GRPCService) ServerStreamHandler(_ any, stream grpc.ServerStream) error {
	ctx := stream.Context()
	tStream := grpc.ServerTransportStreamFromContext(ctx)
	serviceName, methodName, err := parseGRPCMethod(tStream.Method())
	if err != nil {
		slog.ErrorContext(ctx, "Invalid method format", slog.String("method", tStream.Method()))
		return status.Error(codes.InvalidArgument, "Invalid method format")
	}

	slog.InfoContext(ctx, "Received server side streaming gRPC call", slog.String("service", serviceName), slog.String("method", methodName))

	service, ok := s.sdMap[serviceName]
	if !ok {
		slog.ErrorContext(ctx, "No stub found", slog.String("service", serviceName))
		return status.Error(codes.Unimplemented, "Service "+serviceName+" not found")
	}

	method := service.Methods().ByName(protoreflect.Name(methodName))
	if method == nil {
		return status.Error(codes.Unimplemented, "Method "+methodName+" not found")
	}
	input := dynamicpb.NewMessage(method.Input())
	if err := stream.RecvMsg(input); err != nil {
		slog.ErrorContext(ctx, "Failed to receive input message", slog.String("error", err.Error()))
		return status.Error(codes.InvalidArgument, "Failed to receive input message")
	}

	jsonInput, err := protojson.Marshal(input)
	if err != nil {
		slog.Error("Failed to marshall input", slog.String("error", err.Error()))
		return status.Error(codes.InvalidArgument, "Failed to marshall input")
	}
	slog.InfoContext(ctx, "Received message", slog.String("input", string(jsonInput)))

	resp, ok := s.stubs.Get(serviceName, methodName, jsonInput)
	if !ok {
		slog.ErrorContext(ctx, "No stub configured", slog.String("service", serviceName), slog.String("method", methodName))
		return status.Error(codes.NotFound, "No stub configured")
	}

	if resp.Stream != nil && resp.Stream.Data != nil {
		for _, d := range resp.Stream.Data {
			output := dynamicpb.NewMessage(method.Output())
			if err := protojson.Unmarshal(d, output); err != nil {
				slog.ErrorContext(ctx, "Failed to unmarshal response", slog.String("error", err.Error()))
				return status.Error(codes.Internal, "Failed to unmarshal response")
			}

			if err := stream.SendMsg(output); err != nil {
				slog.ErrorContext(ctx, "Failed to send message", slog.String("error", err.Error()))
				return status.Error(codes.Internal, "Failed to send message")
			}

			if resp.Stream.Delay > 0 {
				slog.InfoContext(ctx, "Sleeping", slog.Int("delay_ms", resp.Stream.Delay))
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(time.Duration(resp.Stream.Delay) * time.Millisecond):
				}
			}
		}
		return nil
	}

	return nil
}

// ClientStreamHandler handles client-side streaming gRPC calls by matching them against
// loaded stubs and returning the corresponding response after the stream is closed.
func (s *GRPCService) ClientStreamHandler(_ any, stream grpc.ServerStream) error {
	ctx := stream.Context()
	tStream := grpc.ServerTransportStreamFromContext(ctx)
	serviceName, methodName, err := parseGRPCMethod(tStream.Method())
	if err != nil {
		slog.ErrorContext(ctx, "Invalid method format", slog.String("method", tStream.Method()))
		return status.Error(codes.InvalidArgument, "Invalid method format")
	}

	slog.InfoContext(ctx, "Received client side streaming gRPC call", slog.String("service", serviceName), slog.String("method", methodName))

	service, ok := s.sdMap[serviceName]
	if !ok {
		slog.ErrorContext(ctx, "No stub found", slog.String("service", serviceName))
		return status.Error(codes.Unimplemented, "service "+serviceName+" not found")
	}

	method := service.Methods().ByName(protoreflect.Name(methodName))
	if method == nil {
		return status.Error(codes.Unimplemented, "method "+methodName+" not found")
	}

	resp, ok := s.stubs.Get(serviceName, methodName, nil)
	if !ok {
		return status.Error(codes.NotFound, "no stub found")
	}

	for {
		input := dynamicpb.NewMessage(method.Input())
		if err := stream.RecvMsg(input); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				slog.InfoContext(ctx, "Stream closed by client")
				break
			}
			if errors.Is(err, io.EOF) {
				slog.InfoContext(ctx, "Stream closed by client")
				break
			}

			slog.ErrorContext(ctx, "Failed to receive input message", slog.String("error", err.Error()))
			return status.Error(codes.InvalidArgument, "failed to receive input message")
		}
		jsonInput, err := protojson.Marshal(input)
		if err != nil {
			slog.Error("Failed to marshall input", slog.String("error", err.Error()))
			return status.Error(codes.InvalidArgument, "failed to marshall input")
		}
		slog.InfoContext(ctx, "Received message", slog.String("input", string(jsonInput)))
	}

	if resp.Data != nil {
		output := dynamicpb.NewMessage(method.Output())

		if err := protojson.Unmarshal(resp.Data, output); err != nil {
			slog.ErrorContext(ctx, "Failed to unmarshal response", slog.String("error", err.Error()))
			return status.Error(codes.Internal, "failed to unmarshal response")
		}

		slog.InfoContext(ctx, "Sending success response", slog.String("output", string(resp.Data)))

		if err := stream.SendMsg(output); err != nil {
			slog.ErrorContext(ctx, "failed to send message", slog.String("error", err.Error()))
			return status.Error(codes.Internal, "failed to send message")
		}

		return nil
	}

	if resp.Code != nil {
		err := status.Error(*resp.Code, resp.Error)
		slog.InfoContext(ctx, "Sending error response", slog.String("error", err.Error()))

		return err
	}

	return nil
}

func parseGRPCMethod(fullMethod string) (string, string, error) {
	parts := strings.Split(fullMethod, "/")
	if len(parts) != 3 || parts[1] == "" || parts[2] == "" {
		return "", "", fmt.Errorf("invalid method: %s", fullMethod)
	}
	return parts[1], parts[2], nil
}
