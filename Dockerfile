# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.26 AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/goldbar ./cmd/goldbar

FROM scratch
COPY --from=builder /out/goldbar /goldbar
ENTRYPOINT ["/goldbar"]
