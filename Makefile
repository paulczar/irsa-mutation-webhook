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

# Podman image tasks
.PHONY: image
image:
	podman build -t $(IMAGE):$(VERSION) .
	podman tag $(IMAGE):$(VERSION) $(IMAGE):latest

.PHONY: push
push: image
	podman push $(IMAGE):$(VERSION)
	podman push $(IMAGE):latest

.PHONY: image-x86
image-x86:
	podman build --platform linux/amd64 -t $(IMAGE)-amd64:$(VERSION) .
	podman tag $(IMAGE)-amd64:$(VERSION) $(IMAGE)-amd64:latest

.PHONY: push-x86
push-x86: image-x86
	podman push $(IMAGE)-amd64:$(VERSION)
	podman push $(IMAGE)-amd64:latest

# Deployment
.PHONY: deploy
deploy:
	oc apply -f deploy/cert-manager-webhook-tls.yaml
	oc apply -f deploy/webhook.yaml

.PHONY: deploy-openshift
deploy-openshift:
	oc apply -f deploy/buildconfig.yaml
	oc apply -f deploy/imagestream.yaml
	oc apply -f deploy/cert-manager-webhook-tls.yaml
	oc apply -f deploy/webhook.yaml

.PHONY: delete-openshift
delete-openshift:
	oc delete -f deploy/buildconfig.yaml
	oc delete -f deploy/imagestream.yaml
	oc delete -f deploy/cert-manager-webhook-tls.yaml
	oc delete -f deploy/webhook.yaml

.PHONY: build-openshift
build-openshift:
	oc start-build irsa-mutation-webhook --from-dir=. --follow

# Development tasks
.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build    - Build the webhook binary"
	@echo "  clean    - Remove build artifacts"
	@echo "  image    - Build Podman image"
	@echo "  push     - Build and push Podman image"
	@echo "  image-x86 - Build x86_64 Podman image on ARM"
	@echo "  push-x86  - Build and push x86_64 Podman image on ARM"
	@echo "  deploy   - Deploy webhook to Kubernetes"
	@echo "  deploy-openshift - Deploy webhook to OpenShift"
	@echo "  delete-openshift - Delete webhook from OpenShift"
	@echo "  build-openshift - Build webhook on OpenShift from local source"
	@echo "  fmt      - Format code"

# Default target
.PHONY: all
all: clean build