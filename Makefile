# Basic variables
BINARY_NAME = irsa-mutation-webhook
IMAGE = kubevirt/irsa-mutation-webhook
VERSION ?= $(shell git describe --tags --always --dirty)

# Build the Go binary
.PHONY: build
build:
	go build -o bin/$(BINARY_NAME) cmd/webhook/main.go

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf bin/

# Docker image tasks
.PHONY: image
image:
	docker build -t $(IMAGE):$(VERSION) .
	docker tag $(IMAGE):$(VERSION) $(IMAGE):latest

.PHONY: push
push: image
	docker push $(IMAGE):$(VERSION)
	docker push $(IMAGE):latest

# Deployment
.PHONY: deploy
deploy:
	kubectl apply -f deploy/cert-manager-webhook-tls.yaml
	kubectl apply -f deploy/webhook.yaml

# Development tasks
.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build    - Build the webhook binary"
	@echo "  clean    - Remove build artifacts"
	@echo "  image    - Build Docker image"
	@echo "  push     - Build and push Docker image"
	@echo "  deploy   - Deploy webhook to Kubernetes"
	@echo "  fmt      - Format code"

# Default target
.PHONY: all
all: clean build 