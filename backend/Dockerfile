# syntax=docker/dockerfile:1
#
# Multi-stage build. The build stage carries the Go toolchain; the runtime stage is
# distroless and holds nothing but the static binary, so there is no shell to exec into and
# nothing to patch for CVEs.
#
# Build one service at a time:
#   docker build --build-arg SERVICE=ingest-api -t pulse/ingest-api:dev ./backend

ARG GO_VERSION=1.26

# ---------- build ----------
FROM golang:${GO_VERSION}-alpine AS build

# git is needed for module downloads over VCS; ca-certificates for HTTPS.
RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Dependencies first: this layer is cached until go.mod or go.sum changes.
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG SERVICE=ingest-api
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETOS=linux
ARG TARGETARCH=amd64

ENV CGO_ENABLED=0

# -trimpath keeps absolute build paths out of the binary; -s -w drop the symbol table and
# DWARF, which halves the image size.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
      -trimpath \
      -ldflags="-s -w \
        -X github.com/nxhawk/pulse-analytics/backend/internal/version.Tag=${VERSION} \
        -X github.com/nxhawk/pulse-analytics/backend/internal/version.Commit=${COMMIT} \
        -X github.com/nxhawk/pulse-analytics/backend/internal/version.BuildTime=${BUILD_TIME}" \
      -o /out/app ./cmd/${SERVICE}

# ---------- runtime ----------
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

COPY --from=build /out/app /app

# nonroot is uid/gid 65532 in the distroless image.
USER nonroot:nonroot

EXPOSE 8080 8081

ENTRYPOINT ["/app"]
