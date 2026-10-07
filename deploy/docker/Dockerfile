# Build from the repository root:
#   docker build -f deploy/docker/Dockerfile -t core-engine .

FROM --platform=$BUILDPLATFORM golang:1.26-alpine@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c AS build

# Fail instead of downloading another Go toolchain during the build.
ENV GOTOOLCHAIN=local CGO_ENABLED=0

WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ARG TARGETOS TARGETARCH
ARG VERSION=dev
RUN GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/engine ./cmd/engine

FROM gcr.io/distroless/static-debian13:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3

COPY --from=build /out/engine /engine
COPY LICENSE NOTICE /licenses/

USER nonroot:nonroot

ENTRYPOINT ["/engine"]
