# Build from the repository root:
#   docker build -f deploy/docker/Dockerfile -t core-engine .

FROM --platform=$BUILDPLATFORM golang:1.26-alpine@sha256:a2012b4a1d1f9f7e9f44e0d39b805a901e856b1dd80b4e6ee4338db547050308 AS build

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
