# build app
FROM golang:1.24.11 AS builder

WORKDIR /src
COPY . .
RUN mkdir -p /app && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/localsend-switch-linux-amd64 .

# build final image
FROM scratch

COPY --from=builder /app/localsend-switch-linux-amd64 /localsend-switch-linux-amd64

ENTRYPOINT ["/localsend-switch-linux-amd64"]