# Linode COSI Driver

[![GitHub](https://img.shields.io/github/license/linode/linode-cosi-driver)](LICENSE.txt)
[![Go Report Card](https://goreportcard.com/badge/github.com/linode/linode-cosi-driver)](https://goreportcard.com/report/github.com/linode/linode-cosi-driver)
[![Static Badge](https://img.shields.io/badge/COSI_Specification-v1alpha1-green)](https://github.com/kubernetes-sigs/container-object-storage-interface-spec/tree/v0.1.0)

The Linode COSI Driver is an implementation of the Kubernetes Container Object Storage Interface (COSI) standard. COSI provides a consistent and unified way to expose object storage to containerized workloads running in Kubernetes. This driver specifically enables integration with Linode Object Storage service, making it easier for Kubernetes applications to interact with Linode's scalable and reliable object storage infrastructure.

- [Linode COSI Driver](#linode-cosi-driver)
  - [Getting Started](#getting-started)
  - [License](#license)
  - [Support](#support)
  - [Contributing](#contributing)

## Getting Started

Follow these steps to get started with Linode COSI Driver:

1. **Prerequisites:**
    1. Install COSI Custom Resource Definitions.
    ```
    kubectl create -k github.com/kubernetes-sigs/container-object-storage-interface-api
    ```

    2. Install COSI Controller.
    ```
    kubectl create -k github.com/kubernetes-sigs/container-object-storage-interface-controller
    ```

2. **Installation:**
    <!-- TODO: write install instructions -->

3. **Usage:**
    <!-- TODO: write usage examples -->

## License

Linode COSI Driver is licensed under the [Apache 2.0](LICENSE) terms. Please review it before using or contributing to the project.

## Support

For any issues, questions, or support, please [create an issue](https://github.com/linode/linode-cosi-driver/issues).

## Contributing

We welcome contributions! If you have ideas, bug reports, or want to contribute code, please check out our [Contribution Guidelines](CONTRIBUTING.md).
