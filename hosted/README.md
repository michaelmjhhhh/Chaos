# Chaos hosted trial API (Go)

The optional hosted service grants each device a limited number of screenshot
names through an OpenAI-compatible Agnes upstream. It holds the provider key
server-side. Both endpoints retain their existing URLs and JSON contracts:

- `POST /api/chat/completions`: bearer `<APP_TOKEN>:<deviceHash>`, pinned model,
  nonstreaming single completion, `X-Chaos-Trial-Remaining` on success; quota
  exhaustion returns 402.
- `GET /api/usage`: same bearer; returns `used`, `limit`, `remaining`,
  `globalSpent`, `globalLimit`.
- `POST /api/usage`: same bearer plus `X-Admin-Token`; resets that device's
  counter. `?global=1` or `?global=true` also resets the current UTC month.

The implementation is [internal/hosted](../internal/hosted), with the server in
[cmd/chaos-hosted](../cmd/chaos-hosted). It uses only Go's standard library and
the [Upstash Redis REST command protocol](https://upstash.com/docs/redis/features/restapi).
There is no Node runtime, npm install or frontend build.

## Configuration

Use the variables in [.env.example](.env.example). Required values are
`APP_TOKEN`, `AGNES_API_KEY`, `UPSTASH_REDIS_REST_URL` and
`UPSTASH_REDIS_REST_TOKEN`. `ADMIN_TOKEN` is optional; without it resets are
disabled. Optional defaults:

| Variable | Default |
| --- | --- |
| `MODEL` | `agnes-2.0-flash` |
| `AGNES_BASE_URL` | `https://apihub.agnes-ai.com/v1` |
| `FREE_LIMIT` | `20` |
| `GLOBAL_MONTHLY_LIMIT` | `100000` |
| `PORT` | `3000` |

Malformed or negative quota limits prevent startup; zero disables the respective
quota. Credentials are trimmed. Device IDs must be 16–256 ASCII letters,
digits, underscores or hyphens, which includes the app's existing SHA-256 IDs.
The app token is public in distributed clients, so device identity is not a
cryptographic authentication factor. The global budget remains the hard cost
ceiling against clients inventing device IDs.

## Local run and deployment

From the **repository root**, export the environment variables, then run:

```sh
go run ./cmd/chaos-hosted
```

For the existing Vercel project, **change Root Directory from `hosted/` to the
repository root** and use the **Go** framework preset. The root
[vercel.json](../vercel.json) builds only `cmd/chaos-hosted`; it does not compile
or run the desktop app. Keep the existing domain and environment variables so
installed clients continue to reach the same endpoints and Redis counters.
Remove any inherited npm install/typecheck build overrides in project settings.

The [official Vercel Go runtime guide](https://vercel.com/docs/functions/runtimes/go)
documents root `go.mod`, the Go preset, custom build commands, `PORT`, and
toolchain selection from `go.mod`. The runtime is beta. Local Linux compilation
is covered, but a Vercel preview build and authenticated smoke test are still
required before promoting a deployment; this migration does not deploy changes.

Existing `dev:<hash>` and `budget:<YYYY-MM>` Redis keys remain compatible.
Do not run old and new proxy versions against the same production quota store
for a sustained period: the old version lacks reservations and can overspend
under concurrent calls. Cut traffic over after checking the preview.

## Quota correctness and limits

A single [Redis EVAL](https://upstash.com/docs/redis/commands/scripting/eval)
checks both counters and reserves one unit of each before contacting the paid
provider. Concurrent requests cannot pass separate read-then-increment checks.
The monthly budget has a 35-day TTL. Successful requests keep both reserved
counters. Failures before transport dispatch and explicit non-2xx provider
rejections refund both counters. After dispatch, an uncertain transport failure
or unreadable, truncated, oversized, or undeliverable successful response refunds
only the device allowance: the global budget remains consumed because the
provider may already have billed that call. Connection failures can therefore
conservatively consume global budget, even when no provider charge occurred.

Reservation IDs make rollback idempotent. Counter generation stamps make an
old in-flight request's refund safe even after an administrative reset, without
subtracting from new usage. Redis failures prevent an upstream request from
starting. If a refund cannot be confirmed, or a process dies after reserving,
quota is conservatively consumed; inspect/reset that device administratively
after resolving the incident. Expiring reservation metadata never releases a
counter automatically, which could otherwise allow a paid call to escape the
global cap. Usage includes requests currently in flight.

Request and response bodies are limited to 4 MiB. A request must contain 1–16
message objects. The server pins `model`, `stream=false`, `n=1` and output tokens
to at most 512 (default 256), stripping `max_completion_tokens` after considering
its requested ceiling. Redirects are not followed with provider credentials.
Upstream calls have a 60-second deadline. Failure statuses and bodies pass
through; oversized or unreadable responses become 502. No provider keys,
request bodies, images or device identifiers are logged.

## Verification

Run from the repository root:

```sh
go test -race ./internal/hosted ./cmd/chaos-hosted
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/chaos-hosted ./cmd/chaos-hosted
```

The HTTP tests cover authentication, admin reset, methods, payload/token bounds,
upstream errors/timeouts, quota denials, failure propagation and concurrent
requests. For the actual Lua integration tests, put `redis-server` on `PATH` or
set `REDIS_SERVER_BIN` to its executable. Tests start an isolated temporary Redis
process behind a local HTTP protocol bridge; they never touch a remote account
or production store. They prove device/global caps under concurrency, idempotent
refunds, concurrent-success preservation, reset races, malformed counters,
month expiration/rollover, and the hard global ceiling after timed-out,
truncated, or oversized upstream responses. Without the executable this integration test explicitly
skips, while the remaining HTTP/protocol tests still run.

After deploying a preview, use test credentials and a test Redis database:

```sh
curl -i -X POST "$HOST/api/chat/completions" \
  -H "Authorization: Bearer $APP_TOKEN:testdevicehash0001" \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"reply with: ok"}]}'

curl "$HOST/api/usage" \
  -H "Authorization: Bearer $APP_TOKEN:testdevicehash0001"
```

Check that the first success reports 19 remaining at the default limit, invalid
credentials get 401, exhausted quotas get 402, and a failed upstream call leaves
usage unchanged. Use the admin reset endpoint only on intended test devices.
