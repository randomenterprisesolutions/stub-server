# Stub Server
Lightweight stub server for HTTP and gRPC on one port. Loads `.proto` files directly (no descriptor sets) and supports unary + streaming gRPC.

# What it is (and isn't)
- File-based stubs for local dev, demos, and simple testing.
- Not a full contract testing/verification tool.

# Comparison
| Tool | HTTP | gRPC | gRPC streaming | File-based stubs | Raw HTTP response files | Request matching | Admin API / UI | Verification |
|-|-|-|-|-|-|-|-|-|
| Stub Server (this) | Yes | Yes | Yes | Yes | Yes | Yes | No | No |
| WireMock | Yes | No | No | Yes | Limited | Yes | Yes | Yes |
| MockServer | Yes | Partial (via gRPC proxying) | Limited | Yes | Limited | Yes | Yes | Yes |
| Imposter (imposter.js) | Yes | Yes | Partial | Yes | Limited | Yes | Yes | Partial |

Notes:
- "Limited/Partial" varies by tool/version; this is a high-level comparison.

# Usage

## Parameters
Flags can also be provided via environment variables (flags take precedence).

| Name | Usage | Required | Default | Env var |
|-|-|-|-|-|
| address | Address to listen on | `false` | `:50051` | `STUB_SERVER_ADDRESS` |
| cert | Path to the `cert` file | `false` | - | `STUB_SERVER_CERT` |
| key | Path to the `key` file | `false` | - | `STUB_SERVER_KEY` |
| proto | Directory containing the `.proto` files | `false` | - | `STUB_SERVER_PROTO` |
| stubs | Directory containing the `.json` gRPC stub files | `false` | - | `STUB_SERVER_STUBS` |
| http | Directory containing the `.json` HTTP stub files | `false` | - | `STUB_SERVER_HTTP` |
| grpc-reflection | Enable gRPC reflection | `false` | `true` | `STUB_SERVER_GRPC_REFLECTION` |

## HTTP stub server
To start the HTTP stub server one needs to specify the path to the HTTP stub dir.
`./stub-server --http ./examples/httpstubs`.

The HTTP stub server supports two stub types:
1. JSON stubs
2. Raw HTTP stubs

### Stub formats
| Format | When to use | Notes |
|-|-|-|
| JSON | Most HTTP responses with structured bodies | Supports exact or regex path matching, plus request header/body matching and headers/status/body in the response. |
| Raw HTTP | Multipart/binary or highly custom responses | Full control over headers/body as a raw HTTP response. |

### JSON

The HTTP JSON stub requires `path` or `regex`, `method`, and `response.status`. Use `method: "*"` to match any HTTP method.

#### Minimal examples
```JSON
{
    "path": "/helloworld",
    "method": "*",
    "response": {
        "status": "200"
    }
}
```

```JSON
{
    "regex": "/users/*",
    "method": "*",
    "response": {
        "status": "200"
    }
}
```

#### Fully specified example
```JSON
{
    "path": "/helloworld",
    "method": "GET",
    "response": {
        "header": {
          "Content-Type":  ["application/json"]
        },
        "body": {"message": "Hello from http stub"},
        "status": 201
    }
}
```

### Raw HTTP
To allow more flexibility the stub-server also support raw HTTP responses provided via file.
This is useful if e.g., `multipart/related`, `multipart/formdata` or some binary response shall be returned. The path relative to the stubdir provides the URL path for which the stub is returned by the server. The last segment of the path before the file is the HTTP method.

#### Example
Given the following directory structure, the server returns the response contained in the `test.http` file, if a `GET` request is sent to `/echo`: 
```
stubs/
  echo/
    GET/
      test.http
```

### Request matching
Request matching can be used to configure the behavior more granular.

##### Header matching
Header expectations can be defined using either exact string equality or a regex pattern.

```json
{
    "path": "/secure",
    "method": "POST",
    "request": {
        "headers": {
            "Authorization": {"exact": "Bearer my-token"},
            "Content-Type":  {"regex": "^application/.*"}
        }
    },
    "response": {"status": 200}
}
```

##### Query parameter matching
Query parameter expectations can be defined using either exact string equality or a regex pattern.

```json
{
    "path": "/search",
    "method": "GET",
    "request": {
        "query": {
            "q": {"exact": "stub-server"},
            "id": {"regex": "[0-9]+"}
        }
    },
    "response": {"status": 200}
}
```

##### Body matching
For JSON requests the body expectations can be defined by either using an `exact` or a `contains` (subset).

- `exact`: The request body must be deeply equal to the provided JSON object.
- `contains`: The request body must contain all keys and values from the provided JSON object (extra fields are ignored).

```json
{
    "path": "/users",
    "method": "POST",
    "request": {
        "body": {
            "contains": {
                "name": "John Doe"
            }
        }
    },
    "response": {"status": 201}
}
```

## gRPC stub server

The gRPC stub requires the `service`, `method` and `output` fields. An optional `request` block enables matching on incoming metadata (headers) and the decoded request message body.

### Unary success example
```JSON
{
    "service": "helloworld.Greeter",
    "method": "SayHello",
    "output": {
        "data": {
            "message": "Hello from proto stub"
        }
    }
}
```

### Unary Error example 
```JSON
{
    "service": "helloworld.Greeter",
    "method": "SayHello",
    "output": {
        "error": {
            "code": 3,
            "message": "Invalid request"
        }
    }
}
```

### Request matching
The optional `request` block supports the same `headers` and `body` matchers as the HTTP stubs. Multiple stubs for the same service and method are allowed and stubs with a `request` matcher take precedence.

#### Header matching
gRPC metadata keys are always lowercase. Each entry in `request.headers` accepts exactly one of `exact` or `regex`.

```JSON
{
    "service": "helloworld.Greeter",
    "method": "SayHello",
    "request": {
        "headers": {
            "authorization": {"exact": "Bearer my-token"}
        }
    },
    "output": {
        "data": {"message": "Hello, authenticated user"}
    }
}
```

#### JSON body matching
The request body is the protobuf message decoded to JSON using the standard proto3 JSON mapping (field names are camelCase; `int64` values are represented as strings). Provide an `exact` or `contains` matcher:

```JSON
{
    "service": "helloworld.Greeter",
    "method": "SayHello",
    "request": {
        "body": {
            "contains": {"name": "World"}
        }
    },
    "output": {
        "data": {"message": "Hello, World"}
    }
}
```

#### Fallback stub (no request matcher)
```JSON
{
    "service": "helloworld.Greeter",
    "method": "SayHello",
    "output": {
        "data": {"message": "Hello, unknown caller"}
    }
}
```

To start the gRPC stub server one needs to specify the path to the gRPC stub directory and the path to the proto files. E.g., `./stub-server --proto ./examples/protos --stubs ./examples/protostubs`

To start HTTP and gRPC server you can combine the two commands:
`./stub-server --proto ./examples/protos --stubs ./examples/protostubs --http ./examples/httpstubs`

### Reflection
gRPC reflection (v1) is enabled by default so tools like `grpcurl` can list and describe services.

# Well-known protos
The binary includes blank imports for common well-known types (`any`, `empty`, `timestamp`, `duration`, `longrunning`) so they can be resolved without bundling `.proto` files. These depend on the generated Go proto packages registering descriptors in the global registry.
If you use other well-known protos, either include the `.proto` files under `--proto` or add a blank import in the main package.

# Quick start
HTTP only:
`./stub-server --http ./examples/httpstubs`

gRPC only:
`./stub-server --proto ./examples/protos --stubs ./examples/protostubs`

Both:
`./stub-server --proto ./examples/protos --stubs ./examples/protostubs --http ./examples/httpstubs`

### Env example
```bash
export STUB_SERVER_HTTP=/stubs/http
export STUB_SERVER_PROTO=/stubs/protos
export STUB_SERVER_STUBS=/stubs/grpc
./stub-server
```


# Docker images
Linux images are published via GoReleaser at `ghcr.io/randomenterprisesolutions/stub-server/cmd`. A Windows Server 2022 (nanoserver) image is also published on tags with the suffix `windows-<tag>`. Tag releases are multi-arch manifests that include both Linux and Windows.

Examples:
`ghcr.io/randomenterprisesolutions/stub-server/cmd:v0.6.0`
`ghcr.io/randomenterprisesolutions/stub-server/cmd:windows-v0.6.0`
