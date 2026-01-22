# Stub Server
The Stub Server is a simple stubs server for HTTP(s) and gRPC.
No need to invoke the proto compiler - the proto files are loaded dynamically.
It supports streaming and unary RPC calls for gRPC.
The HTTP and gRPC server run on the same port.

# What it is (and isn't)
- Lightweight stub server for HTTP and gRPC based on files on disk.
- Loads `.proto` files directly; no precompiled descriptor sets required.
- Designed for local development, demos and simple testing, not contract testing.

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

## HTTP stub server
To start the HTTP stub server one needs to specify the path to the HTTP stub dir.
`./stub-server --http ./examples/httpstubs`.

The HTTP stub server supports two stub types:
1. JSON stubs
2. Raw HTTP stubs

### Stub formats
| Format | When to use | Notes |
|-|-|-|
| JSON | Most HTTP responses with structured bodies | Supports exact or regex path matching, plus headers/status/body. |
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
Matching is based on:
- method (use `method: "*"` to match any HTTP method)
- path (exact or regex)

Query parameters and headers are not currently used for matching.

### Non-goals
- request body matching
- contract validation
- expectation verification

## gRPC stub server

The gRPC stub requires the `service`, `method` and `outputs` fields.

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

To start the gRPC stub server one needs to specify the path to the gRPC stub directory and the path to the proto files. E.g., `./stub-server --proto ./examples/protos --stubs ./examples/protostubs`

To start HTTP and gRPC server you can combine the two commands:
`./stub-server --proto ./examples/protos --stubs ./examples/protostubs --http ./examples/httpstubs`

### Reflection
gRPC reflection (v1) is enabled by default so tools like `grpcurl` can list and describe services.

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

# Comparison
The focus here is file-based stubbing with minimal moving parts.

| Tool | HTTP | gRPC | File-based stubs | Raw HTTP response files | Request body matching | Admin API / UI | Verification |
|-|-|-|-|-|-|-|-|
| Stub Server (this) | Yes | Yes | Yes | Yes | No | No | No |
| WireMock | Yes | No | Yes | Limited | Yes | Yes | Yes |
| MockServer | Yes | Partial (via gRPC proxying) | Yes | Limited | Yes | Yes | Yes |
| Imposter (imposter.js) | Yes | Yes | Yes | Limited | Yes | Yes | Partial |

Limitations by design:
- No request body matching or verification.
- No admin API/UI; stubs are loaded from disk on startup.
