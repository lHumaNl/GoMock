# PRD: GoMock Standalone

## 1. Назначение продукта

GoMock Standalone - это легковесный standalone HTTP mock server на Go, вдохновленный WireMock Standalone, но не являющийся его полной копией.

Основная цель проекта - предоставить быстрый, экономичный по ресурсам и простой в эксплуатации mock server, который читает декларативные mapping-файлы с диска, отвечает на HTTP-запросы согласно правилам матчинга и экспортирует метрики в Prometheus-формате.

Проект должен быть проще WireMock по функциональности, но быстрее, компактнее и удобнее для типовых сценариев локального запуска, тестовых окружений, CI/CD и стендов.

## 2. Продуктовое позиционирование

### 2.1. Что GoMock должен делать

- Запускаться как один standalone binary без JVM и тяжелых runtime-зависимостей.
- Читать mappings из `mappings/*.json`, `mappings/*.yaml`, `mappings/*.yml`.
- Читать response body files из `__files/*`.
- Поддерживать базовые WireMock-like request matchers.
- Возвращать один или несколько вариантов response из одного mapping.
- Поддерживать fixed и random delay.
- Экспортировать Prometheus metrics.
- Быть удобным для запуска в Docker, Kubernetes, локальной разработке и CI.

### 2.2. Что GoMock не должен делать в MVP

- Не должен быть 100% drop-in replacement для WireMock.
- Не должен реализовывать Admin API WireMock.
- Не должен поддерживать runtime reload mappings через HTTP API.
- Не должен реализовывать request journal, near misses, proxy/record mode, scenarios, transformers и Handlebars templating в MVP.
- Не должен считать p95/p99/avg/min/max внутри приложения. Это ответственность Prometheus/Grafana.

## 3. Цели

### 3.1. Функциональные цели

- Запуск mock server из CLI.
- Загрузка mappings и files с диска при старте.
- Поддержка JSON и YAML mapping formats.
- Поддержка базового HTTP request matching.
- Поддержка response variants в одном mapping.
- Поддержка стратегий выбора response: single, sequential, random, weighted.
- Поддержка response delay: fixed и random range.
- Экспорт `/metrics` в Prometheus-формате.
- Экспорт health endpoint.

### 3.2. Нефункциональные цели

- Низкое потребление CPU и памяти.
- Высокая конкурентность обработки запросов.
- Отсутствие data races под нагрузкой.
- Простая модульная архитектура с возможностью расширения.
- Чистый код с использованием ООП-подхода, интерфейсов и паттернов проектирования там, где они оправданы.
- TDD для ключевых компонентов.
- Автоматические unit, integration и e2e тесты для критичных сценариев.
- Линтеры для production-кода.
- Регулярные проверки race detector и benchmark/performance checks.

## 4. Пользовательские сценарии

### 4.1. Локальная разработка

Разработчик описывает mock responses в YAML, запускает GoMock локально и использует его вместо внешнего API.

```bash
gomock --root ./mock --port 8080
```

### 4.2. CI/E2E тесты

Команда кладет mappings в репозиторий тестов, запускает GoMock перед e2e тестами и получает стабильное поведение внешних зависимостей.

### 4.3. Тестовый стенд

GoMock запускается в Docker/Kubernetes как mock внешнего сервиса. Метрики собираются Prometheus, latency и нагрузка отображаются в Grafana.

### 4.4. Нестабильные ответы

Пользователь хочет описать разные варианты ответа в одном mapping: например 90% успешных ответов, 10% ошибок или последовательные страницы/состояния.

## 5. CLI требования

### 5.1. Базовый запуск

```bash
gomock --root ./mock --port 8080
```

### 5.2. CLI flags MVP

- `--root`: корневая директория mock-конфигурации. По умолчанию `.`.
- `--host`: host для bind. По умолчанию `0.0.0.0`.
- `--port`: HTTP port. По умолчанию `8080`.
- `--metrics-port`: отдельный port для metrics. Опционально. Если не задан, `/metrics` доступен на основном port.
- `--log-level`: `debug`, `info`, `warn`, `error`. По умолчанию `info`.
- `--strict`: fail-fast при неизвестных или неподдержанных полях mapping. По умолчанию `false`.
- `--version`: вывести версию.

### 5.3. Exit codes

- `0`: штатное завершение.
- `1`: ошибка конфигурации, загрузки mappings или запуска сервера.
- `2`: ошибка CLI arguments.

## 6. Структура файлов

GoMock должен использовать WireMock-like layout:

```text
mock-root/
  mappings/
    get-users.yaml
    create-user.json
  __files/
    users.json
    error.json
```

### 6.1. Mappings

Поддерживаемые расширения:

- `.json`
- `.yaml`
- `.yml`

Файлы загружаются рекурсивно или только из первого уровня?

MVP: только первый уровень `mappings/*.{json,yaml,yml}`.

Future: рекурсивная загрузка поддиректорий.

### 6.2. Files

`bodyFileName` должен резолвиться относительно `__files/`.

Path traversal запрещен. Значения вроде `../secret` должны отклоняться при загрузке конфигурации или при построении response.

## 7. Mapping format

### 7.1. Общая YAML-структура

```yaml
id: get-users
name: Get users list
priority: 10

request:
  method: GET
  urlPath: /api/users
  queryParameters:
    active:
      equalTo: "true"
  headers:
    Authorization:
      contains: Bearer

response:
  status: 200
  headers:
    Content-Type: application/json
  bodyFileName: users.json
```

### 7.2. JSON-структура

JSON должен использовать те же поля, что YAML.

```json
{
  "id": "get-users",
  "name": "Get users list",
  "priority": 10,
  "request": {
    "method": "GET",
    "urlPath": "/api/users"
  },
  "response": {
    "status": 200,
    "headers": {
      "Content-Type": "application/json"
    },
    "bodyFileName": "users.json"
  }
}
```

### 7.3. Required fields

- `request` обязателен.
- Должен быть задан ровно один из `response` или `responses`.
- В response должен быть задан `status`, если не задан, default `200` допустим только при явном решении команды. Рекомендация MVP: требовать явный `status`.
- `id` желателен. Если не задан, генерируется из имени файла. Для метрик лучше использовать стабильный `id`.

### 7.4. Priority

Если несколько mappings подходят под запрос, выбирается mapping с меньшим `priority`.

Если priority одинаковый, порядок должен быть детерминированным:

1. По `priority` ascending.
2. По specificity score descending.
3. По `id` ascending.

Specificity score нужен, чтобы более точные mappings выигрывали у более общих.

## 8. Request matching

### 8.1. Поддерживаемые поля request

```yaml
request:
  method: GET
  url: /api/users?active=true
  urlPath: /api/users
  urlPattern: /api/users/.*
  queryParameters: {}
  headers: {}
  bodyPatterns: []
```

### 8.2. URL matchers

Поддержать:

- `url`: точное совпадение path + query string.
- `urlPath`: точное совпадение path без query string.
- `urlPattern`: regexp по full URL или path. В MVP надо зафиксировать семантику: regexp применяется к request URI без scheme/host.

В одном request mapping должен быть задан только один из `url`, `urlPath`, `urlPattern`.

### 8.3. Method matcher

`method` должен сравниваться case-insensitive, но нормализоваться к uppercase.

Пример:

```yaml
request:
  method: POST
```

### 8.4. Header matchers

Поддерживаемые operators:

- `equalTo`
- `contains`
- `matches`
- `absent`

Пример:

```yaml
headers:
  X-Request-Id:
    matches: "^[a-zA-Z0-9-]+$"
  Authorization:
    contains: Bearer
```

Header names должны сравниваться case-insensitive.

### 8.5. Query parameter matchers

Поддерживаемые operators:

- `equalTo`
- `contains`
- `matches`
- `absent`

Пример:

```yaml
queryParameters:
  page:
    equalTo: "1"
  search:
    contains: john
```

### 8.6. Body matchers

MVP operators:

- `contains`
- `equalTo`
- `matchesJsonPath` частично

Пример:

```yaml
bodyPatterns:
  - contains: "userId"
  - equalTo: '{"active":true}'
  - matchesJsonPath: "$.user.id"
```

### 8.7. matchesJsonPath MVP semantics

На первом этапе `matchesJsonPath` должен поддерживать existence check: JSONPath существует в body.

Future:

- JSONPath + expected value.
- JSONPath + matcher.
- Arrays predicates.

### 8.8. Matching result

Matcher должен возвращать не просто bool, а объект результата:

- matched: true/false.
- reason при false.
- score/specificity.

Это упростит диагностику, тесты и будущий near-miss режим.

## 9. Responses

### 9.1. Single response

```yaml
response:
  status: 200
  headers:
    Content-Type: application/json
  body: '{"ok":true}'
```

### 9.2. bodyFileName

```yaml
response:
  status: 200
  headers:
    Content-Type: application/json
  bodyFileName: users.json
```

Rules:

- `body` и `bodyFileName` не должны использоваться одновременно.
- `bodyFileName` резолвится только внутри `__files/`.
- Файл можно читать при старте и держать в памяти для скорости. Для больших файлов нужен configurable limit в будущем.

### 9.3. Delay

#### Fixed delay

```yaml
response:
  status: 200
  delay:
    type: fixed
    value: 500ms
```

#### Random delay

```yaml
response:
  status: 200
  delay:
    type: random
    min: 100ms
    max: 700ms
```

Rules:

- Duration format должен использовать Go duration syntax: `100ms`, `1s`, `2m`.
- `min` должен быть меньше или равен `max`.
- Delay применяется перед отправкой response.

## 10. Response variants

### 10.1. Назначение

Response variants позволяют описать несколько вариантов ответа внутри одного mapping без WireMock scenarios и нескольких mapping objects.

### 10.2. Sequential mode

```yaml
responses:
  mode: sequential
  variants:
    - name: first
      status: 200
      bodyFileName: page-1.json
    - name: second
      status: 200
      bodyFileName: page-2.json
    - name: third
      status: 404
      body: '{"error":"not found"}'
```

Rules:

- После последнего варианта последовательность начинается сначала.
- Состояние sequence хранится в памяти.
- Реализация должна быть concurrency-safe.
- Race detector должен проходить для concurrent sequential access.

### 10.3. Random mode

```yaml
responses:
  mode: random
  variants:
    - name: ok
      status: 200
      body: '{"ok":true}'
    - name: error
      status: 500
      body: '{"error":"failed"}'
```

Rules:

- Каждый запрос выбирает случайный variant.
- Random generator должен быть безопасен при concurrent access или изолирован per selector.

### 10.4. Weighted mode

```yaml
responses:
  mode: weighted
  variants:
    - name: ok
      weight: 90
      status: 200
      body: '{"ok":true}'
    - name: error
      weight: 10
      status: 500
      body: '{"error":"failed"}'
```

Rules:

- Weight должен быть положительным числом.
- Сумма весов не обязана равняться 100.
- Выбор выполняется пропорционально weight.

### 10.5. Variant name

`name` у variant рекомендуется задавать явно. Если не задан, генерируется индекс вроде `variant-0`.

Variant name используется в логах и метриках.

## 11. HTTP behavior

### 11.1. Matched request

Если найден matching mapping:

- выбрать response или response variant;
- применить delay, если есть;
- выставить status;
- выставить headers;
- записать body, если есть;
- записать metrics.

### 11.2. Unmatched request

Если mapping не найден:

- вернуть `404`;
- body должен быть кратким и полезным;
- записать metric `matched="false"`.

Пример:

```json
{
  "error": "No matching stub found"
}
```

## 12. Observability

### 12.1. Metrics endpoint

Endpoint:

```text
GET /metrics
```

Если `--metrics-port` задан, metrics server запускается отдельно. Если не задан, `/metrics` доступен на основном server.

### 12.2. Required metrics MVP

#### Requests total

```text
gomock_requests_total{stub="get-users",variant="ok",method="GET",status="200",matched="true"}
```

Labels:

- `stub`: stable mapping id или generated id.
- `variant`: response variant name или `default`.
- `method`: HTTP method.
- `status`: HTTP response status.
- `matched`: `true` или `false`.

#### Request duration histogram

```text
gomock_request_duration_seconds_bucket{...}
gomock_request_duration_seconds_sum{...}
gomock_request_duration_seconds_count{...}
```

Измеряет фактическое время обработки запроса, включая configured delay и overhead сервера.

Отдельная метрика configured delay не нужна.

#### In-flight requests

```text
gomock_inflight_requests
```

#### Loaded mappings

```text
gomock_mappings_loaded
```

#### Build info

```text
gomock_build_info{version="...",commit="...",go_version="..."} 1
```

### 12.3. Cardinality rules

Запрещено добавлять high-cardinality labels:

- raw URL;
- query string;
- request body;
- header values;
- dynamic user identifiers.

Допустимые labels должны быть ограничены конфигурацией mappings.

### 12.4. Grafana expectations

Панели должны строиться поверх стандартных Prometheus queries:

P95 latency:

```promql
histogram_quantile(
  0.95,
  sum by (le, stub) (
    rate(gomock_request_duration_seconds_bucket[5m])
  )
)
```

RPS:

```promql
sum by (stub, method, status) (
  rate(gomock_requests_total[5m])
)
```

Average latency:

```promql
sum by (stub) (rate(gomock_request_duration_seconds_sum[5m]))
/
sum by (stub) (rate(gomock_request_duration_seconds_count[5m]))
```

## 13. Health endpoints

### 13.1. Liveness

```text
GET /healthz
```

Response:

```json
{"status":"ok"}
```

### 13.2. Readiness

```text
GET /readyz
```

Ready после успешной загрузки mappings и запуска server.

## 14. Logging

### 14.1. Requirements

- Structured logging.
- Log level через CLI.
- На старте логировать root, loaded mappings count, port, metrics port.
- Для unmatched requests логировать method и path без sensitive details.
- Для matched requests debug-level лог с stub и variant.

### 14.2. Sensitive data

Не логировать body, full query string и authorization headers по умолчанию.

## 15. Architecture

### 15.1. Архитектурный стиль

Проект должен использовать зрелую модульную архитектуру с разделением domain/application/infrastructure concerns.

Рекомендуемый стиль: Hexagonal Architecture / Clean Architecture pragmatically applied.

Go не является классическим ООП-языком, поэтому ООП должен выражаться через:

- небольшие интерфейсы;
- структуры с методами;
- инкапсуляцию состояния;
- явные зависимости через constructors;
- композицию вместо наследования.

### 15.2. Пример структуры проекта

```text
cmd/gomock/
  main.go

internal/app/
  app.go
  config.go

internal/domain/mapping/
  mapping.go
  request.go
  response.go
  delay.go

internal/domain/matcher/
  matcher.go
  url_matcher.go
  header_matcher.go
  query_matcher.go
  body_matcher.go

internal/domain/selector/
  selector.go
  single.go
  sequential.go
  random.go
  weighted.go

internal/configloader/
  loader.go
  json_loader.go
  yaml_loader.go
  validator.go

internal/files/
  resolver.go
  store.go

internal/server/
  http_server.go
  handler.go
  health.go

internal/observability/
  metrics.go
  logging.go

internal/testsupport/
  fixtures.go
```

### 15.3. Dependency direction

Domain packages must not depend on HTTP, Prometheus, filesystem or CLI packages.

Allowed direction:

```text
cmd -> app -> server/configloader/observability -> domain
```

Domain logic must be testable without network and filesystem.

## 16. Design patterns

Patterns should be used pragmatically, not mechanically.

### 16.1. Strategy

Use Strategy for:

- delay calculation: fixed/random;
- response variant selection: single/sequential/random/weighted;
- matcher operators: equalTo/contains/matches/absent.

### 16.2. Factory

Use Factory for constructing:

- matchers from mapping config;
- delay strategies;
- response selectors.

### 16.3. Chain of Responsibility / Composite

Request matching can be represented as a list/composite of matchers. All must pass for request to match.

### 16.4. Repository / Store

Use in-memory mapping store loaded at startup.

Future extension can add reloadable store without changing server handler contract.

### 16.5. Adapter

Use adapters for:

- Prometheus client;
- logger;
- filesystem;
- HTTP server.

## 17. Extensibility requirements

The architecture must allow adding in the future:

- Recursive mappings loading.
- Config reload on SIGHUP.
- Admin API.
- Proxy/record mode.
- Additional body matchers.
- Response templating.
- Fault injection.
- HTTPS/mTLS.
- OpenTelemetry tracing.

These features are not MVP, but current design must not block them.

## 18. Testing strategy

### 18.1. General approach

Use TDD for important domain behavior and edge cases.

Do not write tests for every trivial getter or every line of glue code.

Focus tests on behavior that can break product correctness.

### 18.2. Unit tests

Required unit test areas:

- Config parsing JSON/YAML.
- Config validation.
- URL/method/header/query/body matchers.
- JSONPath existence behavior.
- Mapping priority and deterministic selection.
- Delay strategy validation and calculation.
- Sequential selector concurrency safety.
- Random/weighted selector distribution sanity.
- File resolver path traversal protection.
- Metrics label construction without high cardinality values.

### 18.3. Integration tests

Required integration tests:

- Load mappings from temp `mappings/` and `__files/` structure.
- Start HTTP server with loaded mappings.
- Matched request returns expected status, headers and body.
- `bodyFileName` returns content from `__files`.
- Unmatched request returns 404.
- `/metrics` exposes required metrics.
- Fixed/random delay affects observed request duration within tolerance.

### 18.4. E2E tests

Required e2e scenarios:

- Start compiled binary or `go run ./cmd/gomock` against fixture directory.
- Send real HTTP requests.
- Validate matching and response variants.
- Validate metrics endpoint.
- Validate graceful shutdown.

### 18.5. Race tests

Race detector must be run for test suites that touch concurrent behavior:

```bash
go test -race ./...
```

Specific concurrent cases:

- Many concurrent requests against sequential response variants.
- Many concurrent requests against random/weighted variants.
- Metrics recording under concurrent load.

### 18.6. Performance tests

Benchmarks should cover:

- Matching with 10, 100, 1000 mappings.
- Header/query/body matcher cost.
- Response from memory body.
- Response from loaded bodyFileName.
- Sequential/random/weighted variant selection.

Example commands:

```bash
go test -bench=. -benchmem ./...
```

Optional performance comparison against WireMock can be added later as a separate benchmark harness.

## 19. Quality gates

### 19.1. Required local checks

```bash
go test ./...
go test -race ./...
go test -bench=. -benchmem ./...
golangci-lint run ./...
```

Benchmarks should not necessarily fail CI on every run initially, but regressions must be observable.

### 19.2. Linters

Use `golangci-lint` for production code.

Recommended linters:

- `govet`
- `staticcheck`
- `errcheck`
- `ineffassign`
- `unused`
- `gocritic`
- `revive`
- `bodyclose`
- `misspell`
- `unparam`

Tests can be excluded from strict linting where it improves readability. Do not enforce production-level style constraints on test helpers unless they catch real bugs.

### 19.3. Formatting

- `gofmt` required.
- `goimports` recommended.

## 20. Performance requirements

### 20.1. Design principles

- Load and validate mappings once at startup.
- Precompile regex and JSONPath expressions at startup.
- Avoid parsing mapping config on request path.
- Avoid filesystem reads on request path in MVP by loading `bodyFileName` into memory.
- Avoid per-request allocations where practical.
- Avoid global locks on hot path.
- Keep response selection concurrency-safe with minimal contention.

### 20.2. Initial target expectations

Exact numeric SLOs should be established after first implementation baseline.

Initial qualitative targets:

- Significantly lower memory footprint than Java WireMock for equivalent simple mappings.
- Faster cold start.
- Stable latency under concurrent load.
- No race detector failures.

### 20.3. Profiling

Future or advanced mode should allow pprof endpoints behind explicit flag:

```bash
gomock --pprof :6060
```

Do not enable pprof by default.

## 21. Security requirements

- Reject path traversal in `bodyFileName`.
- Do not expose Admin API in MVP.
- Do not log secrets by default.
- Limit response file size in future. MVP can document memory behavior.
- Validate regex at startup.
- Validate duration values at startup.
- Validate unsupported mode/operator at startup.

## 22. Error handling

### 22.1. Startup errors

Invalid mappings should fail startup with clear error:

- file path;
- field path;
- reason.

Example:

```text
mappings/get-users.yaml: response.delay.min must be <= response.delay.max
```

### 22.2. Runtime errors

Runtime errors should be rare because config is validated at startup.

If response file cannot be loaded, startup should fail.

If unexpected runtime error occurs, return `500` and log safe diagnostic message.

## 23. Configuration compatibility

### 23.1. WireMock-like compatibility

GoMock should intentionally support a subset of familiar WireMock fields:

- `request.method`
- `request.url`
- `request.urlPath`
- `request.urlPattern`
- `request.headers`
- `request.queryParameters`
- `request.bodyPatterns`
- `response.status`
- `response.headers`
- `response.body`
- `response.bodyFileName`

### 23.2. GoMock extensions

GoMock-specific extensions:

- YAML-first format.
- `responses.mode`.
- `responses.variants`.
- `delay.type: random` with `min/max`.

## 24. Documentation requirements

Project documentation should include:

- README with quick start.
- Mapping format reference.
- Examples directory.
- Metrics and Grafana query examples.
- Docker usage.
- Development guide.
- Compatibility matrix with WireMock-like fields.

## 25. Docker and deployment

### 25.1. Docker image

Produce small image, preferably distroless or alpine-like if compatible.

Expected usage:

```bash
docker run --rm -p 8080:8080 -v "$PWD/mock:/mock" gomock:latest --root /mock
```

### 25.2. Kubernetes

Support readiness/liveness probes:

- `/healthz`
- `/readyz`

Expose `/metrics` for Prometheus scraping.

## 26. Agentic workflow

### 26.1. Purpose

Development should use an agentic workflow to keep quality high and avoid mixing design, implementation and verification responsibilities.

Agents are used as role-based helpers. Human or lead agent remains responsible for final decisions.

### 26.2. Workflow overview

For each substantial feature:

1. Product clarification.
2. Technical design.
3. TDD test design.
4. Implementation.
5. Code review.
6. Race/performance validation.
7. Documentation update.

### 26.3. Recommended agents and responsibilities

#### Orchestrator

Use for complex multi-step tasks.

Responsibilities:

- Break feature into implementation tasks.
- Delegate to specialized agents.
- Track dependencies.
- Ensure final integration is coherent.

#### Task Decomposer

Use when a feature is large or ambiguous.

Responsibilities:

- Break work into atomic tasks.
- Identify dependencies.
- Define expected outputs and verification steps.

#### Analyst

Use for architecture analysis and compatibility decisions.

Responsibilities:

- Analyze WireMock behavior when compatibility is needed.
- Compare alternative designs.
- Produce recommendation with tradeoffs.

#### Coder

Use for implementation.

Responsibilities:

- Write production Go code.
- Follow existing architecture.
- Keep changes minimal and cohesive.
- Use TDD for critical behavior.

#### Critic

Use before merging substantial changes.

Responsibilities:

- Verify implementation against requirements.
- Challenge architectural decisions.
- Find missing tests, race risks, performance risks and maintainability issues.

#### Code Reviewer

Use for practical code review.

Responsibilities:

- Identify bugs, edge cases and code quality problems.
- Check SOLID/DRY/KISS/TDD compliance pragmatically.

#### Debugger

Use for failing tests, flaky behavior, data races or performance anomalies.

Responsibilities:

- Reproduce issue.
- Find root cause.
- Recommend minimal fix.

#### DevOps Agent

Use for CI/CD, Docker and release workflow.

Responsibilities:

- Configure GitHub Actions or equivalent CI.
- Build Docker image.
- Add race/lint/test checks.
- Add release artifacts.

### 26.4. Feature workflow template

Every non-trivial feature should follow this checklist:

```text
1. Define requirement and non-goals.
2. Add or update PRD/design docs if behavior changes.
3. Write failing unit/integration tests for important behavior.
4. Implement minimal production code.
5. Run focused tests.
6. Run full tests.
7. Run race detector if concurrency is involved.
8. Run linter for production code.
9. Add benchmark if feature affects hot path.
10. Review with critic/code-reviewer.
11. Update examples/docs.
```

### 26.5. TDD workflow

Use TDD especially for:

- matching logic;
- config validation;
- selector behavior;
- concurrency-sensitive sequential selector;
- file resolver security;
- metrics label construction.

TDD cycle:

```text
Red: write a failing test for desired behavior.
Green: implement the smallest correct code.
Refactor: improve design without changing behavior.
```

### 26.6. Review workflow

Before merging:

- Code reviewer checks code quality and bug risks.
- Critic verifies requirements against PRD.
- Debugger is used only if there are failing/flaky tests or race/performance symptoms.
- DevOps agent validates CI, Docker and release changes when infrastructure is touched.

### 26.7. Agent boundaries

- Analyst and critic are read-only roles.
- Coder writes code.
- DevOps changes infrastructure.
- Debugger diagnoses and proposes/fixes root cause when assigned.
- Orchestrator delegates and integrates, but should not silently change scope.

## 27. Milestones

### 27.1. Milestone 0: Project skeleton

- Go module initialized.
- CLI skeleton.
- Basic app structure.
- CI with test/lint.
- README draft.

### 27.2. Milestone 1: Config loading

- Load YAML/JSON mappings.
- Validate config.
- Load `__files` bodies safely.
- Unit tests for parser/validator/resolver.

### 27.3. Milestone 2: Matching engine

- Method/url/header/query/body matchers.
- Priority and deterministic selection.
- Unit tests and integration tests.

### 27.4. Milestone 3: HTTP server

- Serve matched responses.
- Serve body/bodyFileName.
- Unmatched 404.
- Health endpoints.
- Integration tests.

### 27.5. Milestone 4: Response variants and delay

- Single/sequential/random/weighted selector.
- Fixed/random delay.
- Race tests for concurrent selectors.

### 27.6. Milestone 5: Metrics

- `/metrics` endpoint.
- Request counter.
- Request duration histogram.
- In-flight gauge.
- Loaded mappings gauge.
- Grafana query examples.

### 27.7. Milestone 6: E2E and packaging

- E2E tests with real process.
- Dockerfile.
- Release build script.
- Benchmark baseline.

## 28. Open questions

- Should `status` be required or default to `200`?
- Should mappings be loaded recursively in MVP or only first level?
- Should unmatched response body be JSON always or configurable?
- Which JSONPath library should be used?
- Should response files always be loaded into memory, or should large file streaming be supported early?
- Should `/metrics` be suppressible from normal matching, especially when metrics share main port?
- Should there be a compatibility mode for exact WireMock field names where they differ from GoMock extensions?

## 29. MVP acceptance criteria

MVP is acceptable when:

- GoMock starts from CLI with a root directory.
- YAML and JSON mappings are loaded and validated.
- `__files` body files are supported safely.
- Required request matchers work.
- Single and variant responses work.
- Fixed and random delays work.
- `/healthz`, `/readyz`, `/metrics` work.
- Unit, integration and e2e tests cover critical behavior.
- `go test ./...` passes.
- `go test -race ./...` passes.
- `golangci-lint run ./...` passes for production code.
- Benchmarks exist for matching and selector hot paths.
- README documents quick start, mapping examples and metrics examples.
