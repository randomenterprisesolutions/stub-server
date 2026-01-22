package grpcstub_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randomenterprisesolutions/stub-server/internal/grpcstub"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

func TestReflectionListsServices(t *testing.T) {
	t.Parallel()

	protoDir, stubDir := setupProtoAndStub(t, "foo", fooProto, fooStub)

	conn, cleanup := startBufConnServer(t, protoDir, stubDir)
	t.Cleanup(cleanup)

	services := listServices(t, conn)
	require.Contains(t, services, "foo.Foo")
}

func TestProtoRegistryIsolation(t *testing.T) {
	t.Parallel()

	protoDirA, stubDirA := setupProtoAndStub(t, "foo", fooProto, fooStub)
	protoDirB, stubDirB := setupProtoAndStub(t, "bar", barProto, barStub)

	connA, cleanupA := startBufConnServer(t, protoDirA, stubDirA)
	t.Cleanup(cleanupA)
	connB, cleanupB := startBufConnServer(t, protoDirB, stubDirB)
	t.Cleanup(cleanupB)

	servicesA := listServices(t, connA)
	require.Contains(t, servicesA, "foo.Foo")
	require.NotContains(t, servicesA, "bar.Bar")

	servicesB := listServices(t, connB)
	require.Contains(t, servicesB, "bar.Bar")
	require.NotContains(t, servicesB, "foo.Foo")
}

func setupProtoAndStub(t *testing.T, name, proto, stub string) (string, string) {
	t.Helper()

	root := t.TempDir()
	protoDir := filepath.Join(root, "protos", name)
	stubDir := filepath.Join(root, "stubs", name)

	require.NoError(t, os.MkdirAll(protoDir, 0o755))
	require.NoError(t, os.MkdirAll(stubDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(protoDir, name+".proto"), []byte(proto), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(stubDir, name+".json"), []byte(stub), 0o644))

	return protoDir, stubDir
}

func startBufConnServer(t *testing.T, protoDir, stubDir string) (*grpc.ClientConn, func()) {
	t.Helper()

	srv, err := grpcstub.NewServer(protoDir, stubDir)
	require.NoError(t, err)

	listener := bufconn.Listen(bufSize)
	go func() {
		_ = srv.Serve(listener)
	}()

	dialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}

	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(dialer), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	return conn, func() {
		_ = conn.Close()
		srv.Stop()
		_ = listener.Close()
	}
}

func listServices(t *testing.T, conn *grpc.ClientConn) []string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)

	client := reflectionv1.NewServerReflectionClient(conn)
	stream, err := client.ServerReflectionInfo(ctx)
	require.NoError(t, err)

	err = stream.Send(&reflectionv1.ServerReflectionRequest{
		MessageRequest: &reflectionv1.ServerReflectionRequest_ListServices{
			ListServices: "*",
		},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)

	services := resp.GetListServicesResponse().GetService()
	names := make([]string, 0, len(services))
	for _, svc := range services {
		names = append(names, svc.GetName())
	}

	_ = stream.CloseSend()
	return names
}

const fooProto = `syntax = "proto3";
package foo;

service Foo {
  rpc Ping(PingRequest) returns (PingResponse);
}

message PingRequest {
  string message = 1;
}

message PingResponse {
  string message = 1;
}
`

const barProto = `syntax = "proto3";
package bar;

service Bar {
  rpc Pong(PongRequest) returns (PongResponse);
}

message PongRequest {
  string message = 1;
}

message PongResponse {
  string message = 1;
}
`

const fooStub = `{
  "service": "foo.Foo",
  "method": "Ping",
  "output": {
    "data": {
      "message": "ok"
    }
  }
}
`

const barStub = `{
  "service": "bar.Bar",
  "method": "Pong",
  "output": {
    "data": {
      "message": "ok"
    }
  }
}
`
