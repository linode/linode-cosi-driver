# Linode COSI Driver

[![GitHub](https://img.shields.io/github/license/linode/linode-cosi-driver)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/linode/linode-cosi-driver)](https://goreportcard.com/report/github.com/linode/linode-cosi-driver)
[![Static Badge](https://img.shields.io/badge/COSI_Specification-v1alpha1-green)](https://github.com/kubernetes-sigs/container-object-storage-interface-spec/tree/v0.1.0)

The Linode COSI Driver is an implementation of the Kubernetes Container Object Storage Interface (COSI) standard. COSI provides a consistent and unified way to expose object storage to containerized workloads running in Kubernetes. This driver specifically enables integration with Linode Object Storage service, making it easier for Kubernetes applications to interact with Linode's scalable and reliable object storage infrastructure.

- [Linode COSI Driver](#linode-cosi-driver)
  - [Getting Started](#getting-started)
  - [Configuration](#configuration)
    - [BucketClass](#bucketclass)
    - [BucketAccessClass](#bucketaccessclass)
  - [License](#license)
  - [Support](#support)
  - [Contributing](#contributing)

## Getting Started

Follow these steps to get started with Linode COSI Driver:

1. **Prerequisites:**
    1. Install COSI Custom Resource Definitions.
    ```sh
    kubectl create -k 'github.com/kubernetes-sigs/container-object-storage-interface/?ref=v0.2.0'
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
                    path: BucketInfo # this is JSON formatted document
    ```

## Configuration

Here’s the updated table with descriptions added:

### BucketClass

| Parameter                   | Default    | Values                                                                                               | Description                                                                            |
|-----------------------------|------------|------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------|
| `cosi.linode.com/v1/region` |            | https://techdocs.akamai.com/linode-api/reference/get-object-storage-endpoints                        | **REQUIRED** The region where the object storage bucket will be created.               |
| `cosi.linode.com/v1/acl`    | `private`  | `private`, `public-read`, `authenticated-read`, `public-read-write`                                  | The access control list (ACL) policy that defines who can read or write to the bucket. |
| `cosi.linode.com/v1/cors`   | `disabled` | `disabled`, `enabled`                                                                                | Enables or disables Cross-Origin Resource Sharing (CORS) for the bucket.               |
| `cosi.linode.com/v1/policy` |            | https://techdocs.akamai.com/cloud-computing/docs/define-access-and-permissions-using-bucket-policies | Defines custom bucket policies for fine-grained access control and permissions.        |

### BucketAccessClass

| Parameter                        | Default     | Values                    | Description                                                                                                             |
|----------------------------------|-------------|---------------------------|-------------------------------------------------------------------------------------------------------------------------|
| `cosi.linode.com/v1/permissions` | `read_only` | `read_only`, `read_write` | Defines the access permissions for the bucket, specifying whether users can only read data or also write to the bucket. |

## License

Linode COSI Driver is licensed under the [Apache 2.0](LICENSE) terms. Please review it before using or contributing to the project.

## Support

For any issues, questions, or support, please [create an issue](https://github.com/linode/linode-cosi-driver/issues).

## Contributing

We welcome contributions! If you have ideas, bug reports, or want to contribute code, please check out our [Contribution Guidelines](CONTRIBUTING.md).
