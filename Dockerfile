# Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /build

# Copy module files first so Docker caches the dependency layer.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /aggregator ./cmd/aggregator

# Stage 2: Minimal runtime image
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /aggregator /aggregator

ENTRYPOINT ["/aggregator"]
