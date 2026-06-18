# GoMock

GoMock is a lightweight WireMock-compatible mock server written in Go. It loads mappings from disk, serves deterministic or variant responses, supports common WireMock request matching patterns, and exposes Prometheus metrics.

## Features

- Single standalone binary with no JVM dependency.
- JSON5-compatible `.json` mappings plus YAML/YML support.
- Safe `bodyFileName` loading from `__files/` with path traversal protection.
- Deterministic mapping selection by priority, specificity, and ID.
- Response variants with `sequential`, `random`, and `weighted` modes.
- Fixed and random delays, including common WireMock delay formats.
- Response templating for practical WireMock migration helpers.
- Health, readiness, and Prometheus metrics endpoints.
- Structured logs plus optional verbose traffic logging.

## Installation

Download the archive for your platform from the [releases page](https://github.com/lHumaNl/GoMock/releases), unpack it, and place `gomock` on your `PATH`.

Build from source:

```bash
git clone https://github.com/lHumaNl/GoMock.git
cd GoMock
make build
./bin/gomock --version
```

## Quick Start

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

gomock --root ./mock --host 127.0.0.1 --port 8080
curl 'http://127.0.0.1:8080/api/users?active=true'
```

You can also run directly from source:

```bash
go run ./cmd/gomock --root ./mock --host 127.0.0.1 --port 8080
```

## Directory Layout

GoMock expects a WireMock-style directory structure:

```text
mock/
  mappings/
    get-users.yaml
    create-user.json
  __files/
    users.json
    error.json
```

- `mappings/` is loaded from the first level only.
- Supported mapping extensions: `.json`, `.yaml`, `.yml`.
- `.json` files are JSON5-compatible: comments, trailing commas, single quotes, and unquoted object keys are allowed.
- A JSON file may contain either one mapping object or a top-level `{ "mappings": [...] }` object.

## CLI Reference

```text
--root                    mock root directory (default: .)
--host                    HTTP bind host (default: 0.0.0.0)
--port                    HTTP bind port (default: 8080)
--metrics-port            optional separate metrics port
--log-level               debug, info, warn, or error
--strict                  reject unknown mapping fields
--verbose                 off, summary, or full
--verbose-preview-limit   max request URI characters in summary logs
--verbose-body-limit      max body bytes captured in full logs
--verbose-redact          redact sensitive traffic log fields
--version                 print version and exit
```

## Mapping Examples

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

Add `response-template` to render WireMock-style templates in `body`, preloaded `bodyFileName` content, and response header values.

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

Supported helpers include `jsonPath`, `randomInt`, `randomValue`, `#each`, and `#unless @last`.

## Observability

GoMock exposes:

- `/healthz`
- `/readyz`
- `/metrics`

Key metrics:

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

### Verbose traffic logging warning

`--verbose=summary|full` is useful for debugging, but it can log request and response data. Redaction is **off by default**. Use `--verbose-redact=true` whenever traffic may include secrets, tokens, cookies, passwords, personal data, or production-like payloads.

## Docker

```bash
docker build -t gomock:local .
docker run --rm -p 8080:8080 -v "$PWD/mock:/mock:ro" gomock:local --root /mock
```

Dedicated metrics port:

```bash
docker run --rm \
  -p 8080:8080 -p 9090:9090 \
  -v "$PWD/mock:/mock:ro" \
  gomock:local --root /mock --metrics-port 9090
```

The image runs as a non-root user and mounts mappings from `/mock` by default.

## WireMock Compatibility

Supported matching and response capabilities include:

- `request.method`, `url`, `urlPath`, `urlPathTemplate`, `urlPattern`, `urlPathPattern`
- `headers`, `queryParameters`, `cookies`, `pathParameters`
- `basicAuth` / `basicAuthCredentials`
- `bodyPatterns`: `contains`, `equalTo`, `matches`, `doesNotContain`, `doesNotMatch`, `matchesJsonPath`, `matchesXPath`
- Static response headers, `body`, `base64Body`, `bodyFileName`
- Top-level WireMock `mappings` arrays
- `response-template` for common migration helpers

Compatibility notes:

- `matchesJsonPath` currently performs existence checks only.
- Regex uses Go RE2 first, with a compatibility fallback for common WireMock lookarounds.
- `hasExactly` and `includes` are supported for multi-value header, query, and cookie matching.
- Lower `priority` values win; omitted priority defaults to `5`.
- Some WireMock features, such as the Admin API, are not implemented.

## Performance

Run the test suite:

```bash
make test
```

Run the opt-in local performance smoke test:

```bash
make perf
# or
GOMOCK_PERF=1 go test ./test/perf -run TestPerformanceSmoke -count=1 -v
```

Tune a local load run:

```bash
GOMOCK_PERF_MAPPINGS=500 \
GOMOCK_PERF_CONCURRENCY=64 \
GOMOCK_PERF_DURATION=15s \
./scripts/perf-smoke.sh
```

Benchmark the HTTP server:

```bash
go test ./test/perf -run '^$' -bench BenchmarkHTTPServerLoad -benchmem
```

## Troubleshooting

- Validation errors include the source file, field, and reason.
- `--strict` helps catch unknown fields in JSON5 mappings.
- `request.url` matches path plus query string; `request.urlPath` matches path only.
- `pathParameters` require `urlPathTemplate`.
- `body`, `base64Body`, and `bodyFileName` are mutually exclusive.
- Unmatched requests return:

```json
{"error":"No matching stub found","method":"GET","path":"/missing"}
```

## Development

```bash
make test   # go test ./...
make race   # go test -race ./...
make lint   # golangci-lint run ./...
make fmt    # gofmt all Go files
make tidy   # go mod tidy
make bench  # go test -bench=. -benchmem ./...
make perf   # opt-in local load smoke via scripts/perf-smoke.sh
make build  # build ./bin/gomock for the current GOOS/GOARCH
```

Cross-build example:

```bash
make build GOOS=linux GOARCH=amd64
```

Create release archives:

```bash
VERSION=0.1.0 ./scripts/build-release.sh
```
