# GoMock Standalone

GoMock Standalone is a lightweight WireMock-like mock server written in Go. It reads declarative mapping files from disk, serves HTTP responses, supports response variants and delays, and exports Prometheus metrics.

## Current scope

- Go module and `cmd/gomock` CLI skeleton.
- Clean package layout for app, domain, config loading, and file resolution.
- First-level loading from JSON5-compatible `mappings/*.json`, `mappings/*.yaml`, and `mappings/*.yml`.
- Stable generated mapping IDs from file names when `id` is omitted; WireMock `mappings` arrays include the item index/name to keep IDs unique.
- Startup validation with file path and clear field reasons.
- Safe `bodyFileName` loading from `__files` with path traversal protection.
- Pure matching engine for method, URL, header, query, and body matchers.
- Deterministic mapping selection by priority ascending, specificity descending, and ID ascending.
- WireMock-like default priority of `5` when `priority` is omitted; explicit lower numbers win.
- HTTP endpoints for stubs, `/healthz`, `/readyz`, and `/metrics`.
- Prometheus metrics for request counts, duration, in-flight requests, loaded mappings, and build info.
- WireMock-style `response-template` rendering for common migration helpers.

Admin API and runtime reload are intentionally out of scope for the MVP.

## Quick start

```bash
mkdir -p mock/mappings mock/__files
cat > mock/__files/users.json <<'JSON'
{"users":[{"id":1,"name":"Ada"}]}
JSON
cat > mock/mappings/get-users.yaml <<'YAML'
id: get-users
request:
  method: GET
  urlPath: /api/users
  queryParameters:
    active:
      equalTo: "true"
response:
  status: 200
  headers:
    Content-Type: application/json
  bodyFileName: users.json
YAML

go run ./cmd/gomock --root ./mock --host 127.0.0.1 --port 8080
curl 'http://127.0.0.1:8080/api/users?active=true'
```

Build a local binary when you want a standalone executable:

```bash
make build
./bin/gomock --root ./mock --port 8080
```

## Mapping examples

Mappings live in the first level of `mappings/` and may be JSON, YAML, or YML. Response files are loaded from `__files/` during startup.

`.json` mapping files are JSON5-compatible to ease migration from WireMock exports and hand-maintained stubs. They accept `//` and `/* */` comments, trailing commas, single-quoted strings, and unquoted object keys. A file may contain either one mapping object or a WireMock-style top-level `{ "mappings": [...] }` object; each array item is loaded as a separate mapping. YAML parsing is unchanged. Use `--strict` when you want startup to reject unknown mapping fields after JSON5 decoding.

### Inline response

```yaml
id: create-user
request:
  method: POST
  urlPath: /api/users
  headers:
    Content-Type:
      contains: application/json
  bodyPatterns:
    - matchesJsonPath: $.name
response:
  status: 201
  headers:
    Content-Type: application/json
  body: '{"created":true}'
```

### XML/SOAP body matching

```yaml
id: soap-customer
request:
  method: POST
  urlPath: /soap
  bodyPatterns:
    - matchesXPath: "//*[local-name()='cus' and normalize-space(text())!='']"
response:
  status: 200
  body: '<ok>true</ok>'
```

### Sequential variants

```yaml
id: paged-users
request:
  method: GET
  urlPath: /api/pages
responses:
  mode: sequential
  variants:
    - name: first
      status: 200
      body: '{"page":1}'
    - name: second
      status: 200
      body: '{"page":2}'
```

### Weighted variants with delay

```yaml
id: unstable-api
request:
  method: GET
  urlPath: /api/unstable
responses:
  mode: weighted
  variants:
    - name: ok
      weight: 90
      status: 200
      body: '{"ok":true}'
      delay:
        type: fixed
        value: 50ms
    - name: error
      weight: 10
      status: 503
      body: '{"error":"unavailable"}'
```

### Response templates

Add `response-template` to a response's `transformers` list to render Handlebars-like WireMock templates in `body`, preloaded `bodyFileName` content, and response header values. Responses without this transformer are served unchanged, even when they contain `{{...}}` text.

```yaml
id: templated-callback
request:
  method: POST
  urlPath: /callback
response:
  status: 200
  headers:
    Content-Type: application/json
  transformers:
    - response-template
  body: |
    {
      "path": "{{jsonPath originalRequest.body '$.path'}}",
      "items": [
        {{#each (jsonPath originalRequest.body '$.array') as |item|}}
        {"field":"{{jsonPath item '$.field'}}"}{{#unless @last}},{{/unless}}
        {{/each}}
      ],
      "attempt": {{randomInt lower=1 upper=100}},
      "code": "{{randomValue length=6 type='NUMERIC'}}"
    }
```

Supported helpers are intentionally small and migration-focused: `jsonPath originalRequest.body '$.path'`, `jsonPath item '$.field'` inside loops, `randomInt lower=N upper=N`, `randomValue length=N type='NUMERIC|ALPHABETIC|ALPHANUMERIC'`, `#each (...) as |item|`, and `#unless @last`. Template parse/evaluation failures return `500` for the matched stub.

## Matching engine notes

- `request.method` matching is case-insensitive.
- Header names are matched case-insensitively.
- `url` matches the exact request URI path and query string.
- `urlPath` matches only the path and ignores the query string.
- `urlPattern` applies a regular expression to the request URI without scheme or host. GoMock uses Go's RE2 engine for RE2-compatible patterns and falls back to a compatibility engine for WireMock-style lookarounds such as `(?!...)`.
- `matchesJsonPath` currently performs an existence check only.
- JSONPath evaluation uses `github.com/ohler55/ojg` because it provides a small Go-native parser/evaluator and lets the domain matcher stay independent of HTTP and filesystem concerns.
- `matchesXPath` performs an XML XPath node existence check only. It parses the request body as XML and matches when the expression returns at least one node. XPath evaluation uses `github.com/antchfx/xmlquery`, which supports common WireMock SOAP migration expressions including `local-name()` and `normalize-space()` predicates.
- Lower `priority` values are selected first. If `priority` is omitted, GoMock uses `5`, matching WireMock's default-priority behavior. An explicit `priority: 0` is preserved and wins over omitted priorities.

When multiple mappings match at the same priority, GoMock picks the most specific mapping by matcher score and then the lexicographically smallest mapping ID for deterministic tie-breaking.

## Unmatched requests

Requests that do not match any stub return `404` with a JSON body:

```json
{"error":"No matching stub found","method":"GET","path":"/missing"}
```

The unmatched response is intentionally fixed for now; custom unmatched responses and an Admin API are outside the MVP scope.

## Logging and errors

GoMock writes structured JSON logs to stderr. Use `--log-level debug|info|warn|error` to control verbosity. The default level is `info`.

- `debug`: matched request details.
- `info`: startup and mapping-load summary.
- `warn`: unmatched requests and recoverable request read failures.
- `error`: startup or response-selection failures.

Mapping load and validation failures include the source file, field, and reason. The loader exposes `configloader.ConfigError` for callers that need structured error details.

## Docker usage

Build and run the image locally:

```bash
docker build -t gomock:local .
docker run --rm -p 8080:8080 -v "$PWD/mock:/mock:ro" gomock:local --root /mock
```

Use a separate metrics port when Prometheus should scrape a dedicated listener:

```bash
docker run --rm \
  -p 8080:8080 -p 9090:9090 \
  -v "$PWD/mock:/mock:ro" \
  gomock:local --root /mock --metrics-port 9090
```

The image runs as a non-root user and expects mappings to be mounted at `/mock` unless you pass a different `--root`.

## Metrics and Grafana examples

Required metrics are exported in Prometheus format:

- `gomock_requests_total{stub,variant,method,status,matched}`
- `gomock_request_duration_seconds{stub,variant,method,status,matched}`
- `gomock_inflight_requests`
- `gomock_mappings_loaded`
- `gomock_build_info{version,commit,go_version}`

Example PromQL:

```promql
histogram_quantile(0.95, sum by (le, stub) (rate(gomock_request_duration_seconds_bucket[5m])))
sum by (stub, method, status) (rate(gomock_requests_total[5m]))
sum by (stub) (rate(gomock_request_duration_seconds_sum[5m])) / sum by (stub) (rate(gomock_request_duration_seconds_count[5m]))
```

The server exposes `/metrics` on the main port by default. Use `--metrics-port` to run a separate metrics listener.

Example scrape output includes bounded labels from mapping configuration only:

```text
gomock_requests_total{stub="get-users",variant="default",method="GET",status="200",matched="true"} 1
gomock_mappings_loaded 2
gomock_build_info{version="dev",commit="unknown",go_version="go1.23.0"} 1
```

## Compatibility matrix

| Field or feature | GoMock MVP | Notes |
| --- | --- | --- |
| `request.method` | Supported | Case-insensitive matching, normalized to uppercase. |
| `request.url` | Supported | Exact path plus query string. |
| `request.urlPath` | Supported | Exact path without query string. |
| `request.urlPattern` | Supported | Regex against request URI; RE2 fast path with compatibility fallback for lookarounds. |
| `request.headers` | Supported | `equalTo`, `contains`, `matches`, and `absent`. |
| `request.queryParameters` | Supported | `equalTo`, `contains`, `matches`, and `absent`. |
| `request.bodyPatterns.contains` | Supported | String containment check. |
| `request.bodyPatterns.equalTo` | Supported | Exact body string match. |
| `request.bodyPatterns.matches` | Supported | Regex against the full body string; same regex compatibility behavior as `urlPattern`. |
| `request.bodyPatterns.matchesJsonPath` | Partial | Existence check only. |
| `response.status` | Supported | Required for each response or variant. |
| `response.headers` | Supported | Static response headers. |
| `response.body` | Supported | Mutually exclusive with `bodyFileName`. |
| `response.bodyFileName` | Supported | Loaded from `__files/` at startup with traversal protection. |
| `response.transformers[].response-template` | Partial | Renders common WireMock helpers in bodies, body files, and headers. Unsupported helpers fail when rendered. |
| `responses.mode` | GoMock extension | `sequential`, `random`, and `weighted`. |
| Response delay | GoMock extension + partial WireMock | GoMock `delay` supports `fixed` and `random` using Go duration syntax. WireMock `delayDistribution` supports `uniform` with `lower`/`upper` milliseconds. |
| Top-level `mappings` array | Supported | Each item is loaded as a separate mapping; generated IDs use file name plus item index/name. |
| `serveEventListeners` | Not supported | Ignored in default mode as an unknown field; rejected by `--strict`. Webhooks are not executed. |
| WireMock Admin API | Not supported | Intentionally out of MVP scope. |
| Runtime mapping reload | Not supported | Intentionally out of MVP scope. |

## Development commands

```bash
make test   # go test ./...
make race   # go test -race ./...
make lint   # golangci-lint run ./...
make fmt    # gofmt all Go files
make tidy   # go mod tidy
make bench  # go test -bench=. -benchmem ./...
make build  # build ./bin/gomock for the current GOOS/GOARCH
```

Cross-build for a specific target by passing Go toolchain environment values:

```bash
make build GOOS=linux GOARCH=amd64
```

Create release archives for common platforms:

```bash
VERSION=0.1.0 ./scripts/build-release.sh
```

The release script accepts `PLATFORMS`, `OUTPUT_DIR`, `VERSION`, and `COMMIT` environment overrides and produces SHA-256 checksums.

## Mapping validation highlights

- `request` is required.
- Exactly one of `response` or `responses` is required.
- Exactly one of `request.url`, `request.urlPath`, or `request.urlPattern` is required.
- `response.status` and variant `status` are required.
- `body` and `bodyFileName` are mutually exclusive.
- Unsupported operators, modes, invalid regex, invalid delays, and unsafe body file paths fail startup.
