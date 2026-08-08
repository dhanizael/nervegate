# Build Stage
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o nervegate ./cmd/nervegate

# Final Distroless Stage
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=builder /app/nervegate /nervegate
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/nervegate", "serve"]
