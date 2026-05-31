# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder
ARG VERSION=dev
WORKDIR /src

RUN apk add --no-cache git
ENV GOTOOLCHAIN=local

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /switchdeck \
    ./cmd/switchdeck

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /switchdeck /switchdeck
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/switchdeck", "--version"]
ENTRYPOINT ["/switchdeck"]
CMD ["--data", "/data"]
