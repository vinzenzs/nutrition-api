## ADDED Requirements

### Requirement: OpenAPI specification generation

The system SHALL generate an OpenAPI 2.0 specification from annotations on HTTP handler functions, covering every public endpoint exposed under `/products`, `/meals`, and `/summary`.

The generated specification SHALL include, for each endpoint: HTTP method, path, summary, tags, request parameters (path, query, body), success response schema, and documented error responses (at minimum 400, 401, 404 where applicable).

The generated artifacts SHALL be committed to the repository under `docs/` so that building the binary does not require the documentation toolchain.

#### Scenario: Annotated endpoint appears in the spec

- **WHEN** a handler function in `internal/products`, `internal/meals`, or `internal/summary` is registered on the Gin router with `swag` annotations
- **THEN** running `make swag` regenerates `docs/swagger.json`, `docs/swagger.yaml`, and `docs/docs.go` containing an entry for that endpoint with its method, path, summary, parameters, and response schemas

#### Scenario: Regenerated docs match committed docs

- **WHEN** a developer runs `make swag` against an unchanged source tree
- **THEN** the resulting `docs/` files are byte-identical to the committed versions (so a CI diff check would pass)

### Requirement: Interactive documentation endpoint

The system SHALL expose the generated specification and an interactive Swagger UI at runtime when documentation is enabled.

The specification SHALL be served at `GET /swagger/doc.json`. The interactive UI SHALL be served at `GET /swagger/index.html` (and the canonical wildcard route `/swagger/*any`).

#### Scenario: UI available in debug mode

- **WHEN** the server is started with `gin.Mode()` not equal to `gin.ReleaseMode` (e.g., local development)
- **THEN** `GET /swagger/index.html` returns HTTP 200 with the Swagger UI HTML, and `GET /swagger/doc.json` returns the OpenAPI specification as JSON

#### Scenario: UI opt-in for release mode

- **WHEN** the server is started in release mode AND environment variable `SWAGGER_ENABLED=true`
- **THEN** `GET /swagger/index.html` returns HTTP 200

#### Scenario: UI disabled by default in release mode

- **WHEN** the server is started in release mode AND `SWAGGER_ENABLED` is unset or any value other than `true`
- **THEN** `GET /swagger/index.html` returns HTTP 404 and no `/swagger/*` route is registered on the router

### Requirement: Documentation scope and exclusions

The system SHALL document only API consumer-facing endpoints in the OpenAPI specification.

Health and readiness probes (`/healthz`, `/readyz`) SHALL NOT appear in the specification because they are infrastructure endpoints, not consumer APIs.

Authentication middleware behavior (token header requirement) SHALL be reflected in the specification as a security scheme so consumers see which endpoints require which tokens.

#### Scenario: Health endpoints absent from spec

- **WHEN** `docs/swagger.json` is generated
- **THEN** it contains no `paths` entry for `/healthz` or `/readyz`

#### Scenario: Auth scheme documented

- **WHEN** `docs/swagger.json` is generated
- **THEN** it declares a security scheme covering the `X-API-Token` (or current header name) used by the auth middleware, and each authenticated endpoint references that scheme
