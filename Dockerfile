FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o irsa-mutation-webhook cmd/webhook/main.go
FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/irsa-mutation-webhook .
RUN mkdir -p /etc/webhook/certs
EXPOSE 8443
ENTRYPOINT ["/app/irsa-mutation-webhook"]