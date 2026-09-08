# Goichi examples

Runnable examples for [Goichi](https://github.com/goichi-dev/goichi) — one per protocol,
plus a combined server that starts all of them on a single multiplexed port.

## Layout

```
goichi-examples/
├── internal/     shared DTOs and the demo token issuer
├── rest/         routing, binding, validation, OpenAPI docs, JWT groups
├── websocket/    echo + broadcast over ws://
├── graphql/      GraphQL endpoint and playground
├── grpc/         classic gRPC and ConnectRPC on one port
├── mqtt/         embedded MQTT broker, publish from REST
├── mcp/          MCP tools, resources and prompts for AI agents
├── proxy/        reverse proxy: HTTP load balancing, WebSocket and TCP/database
└── full/         every protocol above in one process
```

## Requirements

Go 1.26 or later. The examples build against a published version of the
framework, so cloning this repo is enough:

```bash
git clone https://github.com/goichi-dev/goichi-examples
cd goichi-examples
go run ./rest
```

To try the examples against unreleased framework changes, add a `replace`
pointing at a local checkout:

```
replace github.com/goichi-dev/goichi => ../goichi
```

Keep that local: CI rejects a committed `replace`, because the examples have to
compile for anyone who just runs `go get`.

## Running

```bash
go run ./rest
go run ./websocket
go run ./graphql
go run ./grpc
go run ./mqtt
go run ./mcp
go run ./proxy
go run ./full
```

Every example listens on `127.0.0.1:8080`, so run one at a time.

## What each example serves

| Example | Endpoints | Try it |
|---|---|---|
| `rest` | `/docs`, `/health`, `/login`, `/api/v1/todos` | `curl -X POST 127.0.0.1:8080/login -d '{"username":"admin","password":"password"}'` |
| `websocket` | `ws://127.0.0.1:8080/ws`, `POST /broadcast`, `GET /clients` | connect a WS client, then `curl -X POST 127.0.0.1:8080/broadcast -d 'hi'` |
| `graphql` | `/graphql`, `/graphql/playground` | `curl -X POST 127.0.0.1:8080/graphql -d '{"query":"{ping}"}'` |
| `grpc` | gRPC + ConnectRPC on `:8080` | `grpcurl -plaintext 127.0.0.1:8080 list` |
| `mqtt` | broker on `:1883`, `POST /publish`, `GET /mqtt/stats` | `mosquitto_sub -h 127.0.0.1 -t 'sensors/#' -v` |
| `mcp` | JSON-RPC on `tcp://127.0.0.1:8085`, `GET /mcp/tools` | see [MCP](#mcp) below |
| `proxy` | `/single`, `/balanced`, `/failover`, `/tenant`, `/ws`, `/proxy/stats`; TCP on `:9300` | `curl 127.0.0.1:8080/balanced/x` twice and watch the backend change |
| `full` | all of the above; MCP on `:8085` | `curl 127.0.0.1:8080/docs` |

## MCP

MCP is **not HTTP** — it is newline-delimited JSON-RPC over a raw TCP socket, so
`curl` cannot talk to it. Use any TCP client:

```bash
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n' | nc 127.0.0.1 8085
```

Supported methods: `initialize`, `tools/list`, `tools/call`, `resources/list`,
`prompts/list`. Send one request per line and read one response line back — the
socket stays open for the whole session.

## Reverse proxy

The `proxy` example starts the upstreams it proxies to, so it runs on its own:
two HTTP backends (`:9201`, `:9202`), a WebSocket echo server (`:9203`) and a
line-based TCP service (`:9301`) standing in for a database. In a real
deployment those live elsewhere and only the proxy configuration is yours.

```bash
# 1. one upstream, prefix stripped: /single/info arrives upstream as /info
curl 127.0.0.1:8080/single/info

# 2. load balanced - run it twice, the "backend" field alternates
curl 127.0.0.1:8080/balanced/x

# 3. one target is deliberately dead; health checks route around it
curl 127.0.0.1:8080/failover/x

# 4. ModifyRequest/ModifyResponse inject headers each way
curl -i "127.0.0.1:8080/tenant/x?tenant=acme"   # X-Served-Via, x_tenant_id

# 6. the TCP proxy in front of the stand-in database
printf 'select 1
' | nc 127.0.0.1 9300           # -> OK: SELECT 1
printf 'DROP TABLE users
' | nc 127.0.0.1 9300   # blocked by the hook

curl 127.0.0.1:8080/proxy/stats
```

The backends echo the forwarding headers they received, so you can see that
`X-Forwarded-For` chains and `X-Forwarded-Proto` reports the real scheme.

`/ws` (5) is a WebSocket passthrough: connect any WS client to
`ws://127.0.0.1:8080/ws` and the upstream echoes each message back.

The TCP proxy is a layer-4 byte pipe, which is why the same code fronts
Postgres, MySQL, Redis or SMTP without the framework parsing any of them. Its
hooks work on **byte chunks, not messages** — a chunk boundary is not a protocol
boundary, so a hook that needs whole packets must buffer them itself. It also
performs **no authentication**: the upstream service authenticates, and
`OnConnect` is where connection-level policy such as an IP allow-list belongs.

## Auth

`rest` and `full` sign real HS256 tokens. The signing key comes from `JWT_SECRET`
and falls back to an obvious dev-only value so the examples run out of the box:

```bash
export JWT_SECRET=your-own-secret
```

Demo credentials are `admin` / `password`. Send the token as
`Authorization: Bearer <token>`.

## Notes

- **GraphQL** ships without a schema on purpose, so there is no codegen step. The
  server answers every query with `{"data":{"ping":"pong"}}`. Generate a real
  schema with gqlgen and assign it to `srv.Schema` — see the comment in
  `graphql/main.go`. The playground is enabled there for convenience; it implies
  schema introspection, so turn both off in production.
- **gRPC** registers only the health and reflection services. Swap in your own
  generated `pb.RegisterYourServiceServer` call inside `RegisterFunc`.
- **MQTT** runs on its own TCP port rather than the multiplexed one, so any
  standard MQTT client can connect. The other HTTP-based protocols share `:8080`
  via cmux. `MQTTBroker.Subscribe` lets the broker observe its own traffic
  server-side (with `+` and `#` wildcards); a real client such as
  `mosquitto_sub` is still the easiest way to watch topics by hand.
- **MCP** always opens its own listener, which is why it sits on `:8085` even in
  the multiplexed `full` example.
- **gRPC in `full`** registers no services on purpose, so a call there answers
  `Unimplemented`. The `grpc` example registers health and reflection, which is
  what makes `grpcurl` and health checks work.

## License

MIT — see [LICENSE](LICENSE). Same as the framework.
