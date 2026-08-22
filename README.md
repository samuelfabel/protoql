# ProtoQL

Research prototype: compile GraphQL query selections into a dynamic response projection, encode with Protocol Buffers, and serve over gRPC.

**Status:** proof-of-concept — validating technical feasibility.

## Hypothesis

If clients can define GraphQL response shape, that shape can also define a transient binary serialization contract without a static `.proto` per query.

## Pipeline (target)

```text
GraphQL query → projection plan → in-memory evaluation → dynamic descriptor → protobuf bytes → gRPC
```

## Current milestone

Basic scalar materialization: `Project` maps direct `String`, `ID`, `Int`, `Boolean`, and `Float` bindings to Go values (`internal/engine`).

```bash
go test ./internal/engine/...
```

The query compiler lives in `internal/compile`.

```bash
go test ./internal/compile/...
```

## Development

Requires Go 1.22 or later.

```bash
go test ./...
```

## License

MIT — see [LICENSE](LICENSE).
