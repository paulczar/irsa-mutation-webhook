# IRSA Mutation Webhook for KubeVirt

A Kubernetes mutation webhook that injects a virtio-fs container into KubeVirt pods to share AWS IAM tokens from the host with the virtual machine, enabling secure AWS IAM role access for KubeVirt workloads.

## Overview

This webhook automatically injects a virtio-fs container into KubeVirt pods when the associated service account has IRSA annotations. The virtio-fs container shares the host directory containing the AWS IAM token (`/var/run/secrets/eks.amazonaws.com/serviceaccount`) with the KubeVirt VM, enabling the VM to securely access AWS services using IRSA.

## Features

- Automatic injection of virtio-fs container for IRSA token sharing
- Integration with KubeVirt's virtio-fs mechanism
- Support for AWS IAM role-based authentication in KubeVirt VMs
- Secure token sharing through virtio-fs
- Configurable resource allocation for the virtio-fs container

## Prerequisites

- Go 1.24 or later
- Kubernetes cluster with KubeVirt installed
- AWS EKS cluster with IRSA enabled
- kubectl configured with access to your cluster
- cert-manager installed in your cluster

## Architecture

The webhook works in the following way:

1. Intercepts pod creation requests for KubeVirt workloads
2. Checks for IRSA annotations on the service account
3. If IRSA is configured, injects a virtio-fs container that:
   - Mounts the AWS IAM token directory from the host
   - Shares it with the KubeVirt VM through virtio-fs
   - Configures appropriate resource limits and requests
4. Ensures secure token sharing between host and VM

## Installation

1. Clone the repository:
```bash
git clone https://github.com/kubevirt/irsa-mutation-webhook.git
cd irsa-mutation-webhook
```

2. Build the webhook:
```bash
make build
```

3. Build and push the Docker image:
```bash
make image
make push
```

4. Deploy to Kubernetes:
```bash
make deploy
```

## Configuration

The webhook can be configured through environment variables:

```yaml
env:
  - name: VIRTIOFS_IMAGE
    value: "quay.io/kubevirt/virt-launcher:v1.5.1"
  - name: RESOURCE_REQUESTS_CPU
    value: "100m"
  - name: RESOURCE_REQUESTS_MEMORY
    value: "128Mi"
  - name: RESOURCE_LIMITS_CPU
    value: "200m"
  - name: RESOURCE_LIMITS_MEMORY
    value: "256Mi"
```

### KubeVirt Integration

To enable IRSA for KubeVirt workloads:

1. Annotate your service account with the IAM role:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: example-sa
  annotations:
    eks.amazonaws.com/role-arn: "arn:aws:iam::ACCOUNT_ID:role/kubevirt-role"
```

2. Create a KubeVirt VM with the annotated service account:
```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstance
metadata:
  annotations:
    hooks.kubevirt.io/hookSidecars: |
      [
        {
          "args": ["--version", "v1alpha3"],
          "image": "quay.io/kubevirt/eks-irsa-sidecar"
        }
      ]
  name: example-vmi
spec:
  domain:
    devices:
      filesystems:
        - name: serviceaccount-fs
          virtiofs: {}
      disks:
        - disk:
            bus: virtio
          name: containerdisk
    machine:
      type: ""
    resources:
      requests:
        memory: 1024M
  volumes:
    - name: containerdisk
      containerDisk:
        image: quay.io/containerdisks/fedora:latest
    - cloudInitNoCloud:
        userData: |-
          #cloud-config
          chpasswd:
            expire: false
          password: fedora
          user: fedora
          bootcmd:
            # mount the ConfigMap
            - "sudo mkdir -p /mnt/serviceaccount"
            - "sudo mkdir -p /mnt/aws-iam-token"
            - "sudo mount -t virtiofs serviceaccount-fs /mnt/serviceaccount"
            - "sudo mount -t virtiofs aws-iam-token /mnt/aws-iam-token"
      name: cloudinitdisk
    - name: serviceaccount-fs
      serviceAccount:
        serviceAccountName: example-sa
```

## Usage

1. Create a service account with IRSA annotations
2. Deploy your KubeVirt VM using the service account
3. The webhook will automatically inject the virtio-fs container to share the AWS IAM token

## Development

### Available Make Targets

The project includes several helpful make targets to assist with development:

```bash
# Build the webhook binary
make build

# Clean build artifacts
make clean

# Build Docker image
make image

# Push Docker image to registry
make push

# Deploy webhook to Kubernetes
make deploy

# Format code
make fmt
```

### Running Locally

```bash
./bin/irsa-mutation-webhook
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
