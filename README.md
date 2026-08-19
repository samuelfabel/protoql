# ProtoQL

Research prototype: compile GraphQL query selections into a dynamic response projection, encode with Protocol Buffers, and serve over gRPC.

**Status:** proof-of-concept — validating technical feasibility.

## Hypothesis

If clients can define GraphQL response shape, that shape can also define a transient binary serialization contract without a static `.proto` per query.

## Pipeline (target)

```text
GraphQL query → projection plan → in-memory evaluation → dynamic descriptor → protobuf bytes → gRPC
```

## Development

Requires Go 1.22 or later.

```bash
go test ./...
```

## License

MIT — see [LICENSE](LICENSE).
