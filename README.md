# Linode COSI Driver

[![GitHub](https://img.shields.io/github/license/linode/linode-cosi-driver)](LICENSE.txt)
[![Go Report Card](https://goreportcard.com/badge/github.com/linode/linode-cosi-driver)](https://goreportcard.com/report/github.com/linode/linode-cosi-driver)
[![Static Badge](https://img.shields.io/badge/COSI_Specification-v1alpha1-green)](https://github.com/kubernetes-sigs/container-object-storage-interface-spec/tree/v0.1.0)

The Linode COSI Driver is an implementation of the Kubernetes Container Object Storage Interface (COSI) standard. COSI provides a consistent and unified way to expose object storage to containerized workloads running in Kubernetes. This driver specifically enables integration with Linode Object Storage service, making it easier for Kubernetes applications to interact with Linode's scalable and reliable object storage infrastructure.

- [Linode COSI Driver](#linode-cosi-driver)
  - [Getting Started](#getting-started)
  - [Testing](#testing)
    - [Integration tests](#integration-tests)
      - [Prerequisites](#prerequisites)
      - [Test Execution](#test-execution)
      - [Configuration](#configuration)
      - [Test Cases](#test-cases)
        - [Happy Path Test](#happy-path-test)
      - [Suite Structure](#suite-structure)
  - [License](#license)
  - [Support](#support)
  - [Contributing](#contributing)

## Getting Started

Follow these steps to get started with Linode COSI Driver:

1. **Prerequisites:**
    1. Install COSI Custom Resource Definitions.
    ```sh
    kubectl create -k github.com/kubernetes-sigs/container-object-storage-interface-api
    ```

    2. Install COSI Controller.
    ```sh
    kubectl create -k github.com/kubernetes-sigs/container-object-storage-interface-controller
    ```

2. **Installation:**
    1. Create new API token in [Akamai Cloud Manager](https://cloud.linode.com/profile/tokens). The token must be configured with the following permissions:
        - **Object Storage** - Read/Write

    2. Install Linode COSI Driver using Helm.
    ```sh
    helm install linode-cosi-driver \
        ./helm/linode-cosi-driver/ \
        --set=apiToken=<YOUR_LINODE_API_TOKEN> \
        --namespace=linode-cosi-driver \
        --create-namespace
    ```

3. **Usage:**
    1. Create Bucket Class (see the [example.BucketClass.yaml](./examples/example.BucketClass.yaml)).
    ```sh
    kubectl create -f ./examples/example.BucketClass.yaml
    ```

    2. Create Bucket Access Class (see the [example.BucketAccessClass.yaml](./examples/example.BucketAccessClass.yaml)).
    ```sh
    kubectl create -f ./examples/example.BucketAccessClass.yaml
    ```

    3. Create Bucket Claim (see the [example.BucketClaim.yaml](./examples/example.BucketClaim.yaml)).
    ```sh
    kubectl create -f ./examples/example.BucketClaim.yaml
    ```

    4. Create Bucket Access Class (see the [example.BucketAccess.yaml](./examples/example.BucketAccess.yaml)).
    ```sh
    kubectl create -f ./examples/example.BucketAccess.yaml
    ```

    5. Use the `example-secret` secret in your workload, e.g. in deployment:
    ```yaml
    spec:
      template:
        spec:
          containers:
            - volumeMounts:
                - mountPath: /conf
                  name: example-secret-vol
          volumes:
            - name: example-secret-vol
              secret:
                secretName: example-secret
                items:
                  - key: BucketInfo
                    path: BucketInfo.json
    ```

## Testing

### Integration tests

#### Prerequisites

Before running the integration tests, ensure the following prerequisites are met:

- **Linode Account**: You need a valid Linode account with access to the Linode API.
- **Linode Token**: Set the `LINODE_TOKEN` environment variable with your Linode API token.
- **Environment Variables**: Additional environment variables, such as `LINODE_API_URL` and `LINODE_API_VERSION`, can be set as needed.

#### Test Execution

To run the integration tests, execute the following:

```bash
go test -tags=integration ./...
```

The tests cover various operations such as creating a bucket, granting and revoking bucket access, and deleting a bucket. These operations are performed multiple times to ensure idempotency.

#### Configuration

The test suite provides configurable parameters through environment variables:

- `LINODE_TOKEN`: Linode API token.
- `LINODE_API_URL`: Linode API URL.
- `LINODE_API_VERSION`: Linode API version.
- `IDEMPOTENCY_ITERATIONS`: Number of times to run idempotent operations (default is 2).

#### Test Cases

##### Happy Path Test

The `TestHappyPath` function executes a series of idempotent operations on the Linode COSI driver, covering bucket creation, access granting and revoking, and bucket deletion. The test validates the driver's functionality under normal conditions.

#### Suite Structure

The test suite is organized into a `suite` struct, providing a clean separation of concerns for different test operations. The suite includes methods for creating, deleting, granting access to, and revoking access from a bucket. These methods are called in an idempotent loop to ensure the driver's robustness.

## License

Linode COSI Driver is licensed under the [Apache 2.0](LICENSE) terms. Please review it before using or contributing to the project.

## Support

For any issues, questions, or support, please [create an issue](https://github.com/linode/linode-cosi-driver/issues).

## Contributing

We welcome contributions! If you have ideas, bug reports, or want to contribute code, please check out our [Contribution Guidelines](CONTRIBUTING.md).
