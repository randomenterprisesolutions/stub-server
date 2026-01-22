package handler_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/randomenterprisesolutions/stub-server/internal/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	helloworldpb "google.golang.org/grpc/examples/helloworld/helloworld"
	routeguide "google.golang.org/grpc/examples/route_guide/routeguide"
)

var serverURL string

func TestMain(m *testing.M) {
	server, err := startTestServer("../../examples/httpstubs", "../../examples/protos", "../../examples/protostubs")
	if err != nil {
		slog.Error("failed to start server", slog.String("error", err.Error()))
		os.Exit(1)
	}
	serverURL = server.URL
	defer server.Close()

	fmt.Printf("Test server started at %s\n", server.URL)
	os.Exit(m.Run())
}

func startTestServer(httpDir, protoDir, stubDir string) (*httptest.Server, error) {
	handler, err := handler.New(httpDir, protoDir, stubDir)
	if err != nil {
		return nil, fmt.Errorf("create handler: %w", err)
	}
	server := httptest.NewServer(handler)
	return server, err
}

func TestHTTPServer(t *testing.T) {
	t.Parallel()

	t.Run("URL not found", func(t *testing.T) {
		t.Parallel()

		url, err := url.JoinPath(serverURL, "not-found")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Method not found", func(t *testing.T) {
		t.Parallel()

		url, err := url.JoinPath(serverURL, "helloworld")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, url, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("Found", func(t *testing.T) {
		t.Parallel()

		url, err := url.JoinPath(serverURL, "helloworld")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.JSONEq(t, `{"message": "Hello from http stub"}`, string(body))
	})

	t.Run("Raw HTTP found", func(t *testing.T) {
		t.Parallel()

		url, err := url.JoinPath(serverURL, "echo")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "Some text", string(body))
		assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
		assert.Equal(t, "9", resp.Header.Get("Content-Length"))
		assert.Equal(t, "Wed, 19 Jul 1972 19:00:00 GMT", resp.Header.Get("Date"))
	})

	t.Run("regex found", func(t *testing.T) {
		t.Parallel()

		url, err := url.JoinPath(serverURL, "/users/1234")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodGet, url, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, resp.Body.Close())
		}()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		assert.JSONEq(t, `{"name": "Jane Doe", "birthdate": "20-06-1990"}`, string(body))
	})

	t.Run("regex method mismatch", func(t *testing.T) {
		t.Parallel()

		url, err := url.JoinPath(serverURL, "/users/1234")
		require.NoError(t, err)

		req, err := http.NewRequest(http.MethodPost, url, nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestGrpcServerSuccessResponses(t *testing.T) {
	t.Parallel()

	url, _ := strings.CutPrefix(serverURL, "http://")
	c, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	t.Run("Unary call", func(t *testing.T) {
		t.Parallel()

		client := helloworldpb.NewGreeterClient(c)
		reply, err := client.SayHello(context.TODO(), &helloworldpb.HelloRequest{
			Name: "Jane",
		})

		require.NoError(t, err)
		assert.Equal(t, "Hello from proto stub", reply.Message)
	})

	t.Run("Server side streaming", func(t *testing.T) {
		t.Parallel()

		client := routeguide.NewRouteGuideClient(c)
		stream, err := client.ListFeatures(context.TODO(), &routeguide.Rectangle{
			Lo: &routeguide.Point{Latitude: 400000000, Longitude: -750000000},
			Hi: &routeguide.Point{Latitude: 420000000, Longitude: -730000000},
		})
		require.NoError(t, err)

		results := make([]*routeguide.Feature, 0, 3)
		for {
			feature, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			require.NoError(t, err)
			results = append(results, feature)
		}

		require.Len(t, results, 3)

		assert.Equal(t, "#1", results[0].Name)
		assert.Equal(t, int32(409146138), results[0].Location.Latitude)
		assert.Equal(t, int32(-746188906), results[0].Location.Longitude)

		assert.Equal(t, "#2", results[1].Name)
		assert.Equal(t, int32(413628156), results[1].Location.Latitude)
		assert.Equal(t, int32(-749015468), results[1].Location.Longitude)

		assert.Equal(t, "#3", results[2].Name)
		assert.Equal(t, int32(419999544), results[2].Location.Latitude)
		assert.Equal(t, int32(733555590), results[2].Location.Longitude)
	})

	t.Run("Client side streaming", func(t *testing.T) {
		t.Parallel()

		client := routeguide.NewRouteGuideClient(c)
		stream, err := client.RecordRoute(context.TODO())
		require.NoError(t, err)

		err = stream.Send(&routeguide.Point{Latitude: 20, Longitude: -40})
		require.NoError(t, err)
		err = stream.Send(&routeguide.Point{Latitude: 10, Longitude: -500})
		require.NoError(t, err)
		err = stream.Send(&routeguide.Point{Latitude: 124234, Longitude: -12142352})
		require.NoError(t, err)

		summary, err := stream.CloseAndRecv()
		require.NoError(t, err)
		assert.Equal(t, int32(10), summary.PointCount)
		assert.Equal(t, int32(5), summary.FeatureCount)
		assert.Equal(t, int32(1000), summary.Distance)
		assert.Equal(t, int32(120), summary.ElapsedTime)
	})
}
