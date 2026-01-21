package grpcstub

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bufbuild/protocompile/parser"
	"github.com/bufbuild/protocompile/reporter"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// registerTypes loads all .proto files from the specified directory and registers them
// with the gRPC server.
func (s *GRPCService) registerTypes(protoDir string) error {
	err := filepath.WalkDir(protoDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			if filepath.Ext(path) != ".proto" {
				return nil
			}
			n, err := filepath.Rel(protoDir, path)
			if err != nil {
				return err
			}

			return s.registerProto(protoDir, n)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("register services: %w", err)
	}

	return nil
}

func (s *GRPCService) registerServices() {
	s.files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		for svcNum := 0; svcNum < fd.Services().Len(); svcNum++ {
			svc := fd.Services().Get(svcNum)
			serviceName := string(svc.FullName())
			s.sdMap[serviceName] = svc
			gsd := grpc.ServiceDesc{ServiceName: serviceName, HandlerType: (*interface{})(nil)}
			for methodNum := 0; methodNum < svc.Methods().Len(); methodNum++ {
				m := svc.Methods().Get(methodNum)
				slog.Info("registering gRPC method", slog.String("service", serviceName), slog.String("method", string(m.Name())), slog.Bool("client_stream", m.IsStreamingClient()), slog.Bool("server_stream", m.IsStreamingServer()))
				if m.IsStreamingServer() {
					gsd.Streams = append(gsd.Streams, grpc.StreamDesc{StreamName: string(m.Name()), Handler: s.ServerStreamHandler, ServerStreams: m.IsStreamingServer(), ClientStreams: m.IsStreamingClient()})
					continue
				}
				if m.IsStreamingClient() {
					gsd.Streams = append(gsd.Streams, grpc.StreamDesc{StreamName: string(m.Name()), Handler: s.ClientStreamHandler, ServerStreams: m.IsStreamingServer(), ClientStreams: m.IsStreamingClient()})
					continue
				}
				gsd.Methods = append(gsd.Methods, grpc.MethodDesc{MethodName: string(m.Name()), Handler: s.Handler})
			}
			s.grpcServer.RegisterService(&gsd, s)
		}
		return true
	})
}

func (s *GRPCService) registerProto(protoDir string, protoFileName string) (err error) {
	protoFileName = strings.ReplaceAll(protoFileName, "\\", "/")

	// Skip the file if it is already registered
	if _, err := s.files.FindFileByPath(protoFileName); err == nil {
		return nil
	}

	f, err := os.Open(path.Join(protoDir, protoFileName))
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer func() {
		closeErr := f.Close()
		if closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close file: %w", closeErr))
		}
	}()

	handler := reporter.NewHandler(nil)
	node, err := parser.Parse(protoFileName, f, handler)
	if err != nil {
		return fmt.Errorf("parse proto: %w", err)
	}

	res, err := parser.ResultFromAST(node, true, handler)
	if err != nil {
		return fmt.Errorf("convert from AST: %w", err)
	}

	// recursively register dependencies
	for _, d := range res.FileDescriptorProto().Dependency {
		err = s.registerProto(protoDir, d)
		if err != nil {
			return err
		}
	}

	fd, err := protodesc.NewFile(res.FileDescriptorProto(), s.files)
	if err != nil {
		return fmt.Errorf("convert to FileDescriptor: %w", err)
	}

	if err := s.files.RegisterFile(fd); err != nil {
		return fmt.Errorf("register file: %w", err)
	}

	for i := 0; i < fd.Messages().Len(); i++ {
		msg := fd.Messages().Get(i)
		if err := s.types.RegisterMessage(dynamicpb.NewMessageType(msg)); err != nil {
			return fmt.Errorf("register message %q: %w", msg.FullName(), err)
		}
	}
	for i := 0; i < fd.Extensions().Len(); i++ {
		ext := fd.Extensions().Get(i)
		if err := s.types.RegisterExtension(dynamicpb.NewExtensionType(ext)); err != nil {
			return fmt.Errorf("register extension %q: %w", ext.FullName(), err)
		}
	}

	return nil
}
