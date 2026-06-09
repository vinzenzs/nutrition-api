# syntax=docker/dockerfile:1.7
#
# Multi-stage build:
#   1. golang:1.26-alpine compiles a statically-linked nutrition-api binary
#      with version + commit injected via -ldflags. Migrations and Swagger
#      docs are embedded via embed.FS in the Go sources, so the runtime
#      stage only needs the binary.
#   2. distroless/static-debian12:nonroot runs as UID 65532. No shell, no
#      package manager — the runtime image's only artefact is the binary
#      at /app/nutrition-api.
#
# Build args (passed by CI in release.yml / main.yml; defaulted for local builds):
#   VERSION  — e.g. "v1.2.3", "main-abc1234"; surfaces via `nutrition-api version`
#   COMMIT   — full git SHA; surfaces via `nutrition-api version`

ARG VERSION=dev
ARG COMMIT=unknown

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG VERSION
ARG COMMIT
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 + -trimpath give us a deterministic, statically-linked
# binary suitable for distroless/static. -s -w strip symbol + DWARF tables;
# combined they save ~25% of the binary size.
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath \
        -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
        -o /out/nutrition-api ./cmd/nutrition-api

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/nutrition-api /app/nutrition-api
WORKDIR /app
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/nutrition-api"]
CMD ["serve"]
