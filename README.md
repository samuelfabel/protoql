# ProtoQL

GraphQL query → execution plan → dynamic Protobuf projection → gRPC transport.

## Status

| Milestone | Description |
|-----------|-------------|
| F0-00 | Repository bootstrap |
| F1-01 | Query compiler (GraphQL → execution plan) |
| F1-02 | Projection engine (plan → dynamic record) |
| F2-01 | Basic scalar types (String, Int, Float, Boolean, ID) |
| **F3-01** | **Nested projections and lists** |

## Quick start

```bash
go test ./...
```

## Architecture

```
GraphQL query
    ↓ compile
ExecutionPlan (field bindings)
    ↓ project
Record (map[string]any)
    ↓ encode (future)
Protobuf bytes
    ↓ transport (future)
gRPC
```

## License

MIT — see [LICENSE](LICENSE).
