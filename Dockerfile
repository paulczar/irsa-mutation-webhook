FROM registry.access.redhat.com/ubi9/go-toolset AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /tmp/irsa-mutation-webhook cmd/webhook/main.go
FROM registry.access.redhat.com/ubi9-minimal
WORKDIR /app
COPY --from=builder /tmp/irsa-mutation-webhook .
RUN mkdir -p /etc/webhook/certs
EXPOSE 8443
ENTRYPOINT ["/app/irsa-mutation-webhook"]