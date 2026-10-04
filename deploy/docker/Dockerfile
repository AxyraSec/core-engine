# Build from the repository root:
#   docker build -f deploy/docker/Dockerfile -t core-engine .

FROM --platform=$BUILDPLATFORM golang:1.27-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 AS build

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

USER nonroot:nonroot

ENTRYPOINT ["/engine"]
