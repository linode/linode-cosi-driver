# Linode COSI Driver

[![GitHub](https://img.shields.io/github/license/linode/linode-cosi-driver)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/linode/linode-cosi-driver)](https://goreportcard.com/report/github.com/linode/linode-cosi-driver)
[![Static Badge](https://img.shields.io/badge/COSI_Specification-v1alpha2-green)](https://github.com/kubernetes-sigs/container-object-storage-interface/tree/7b121df3e445b9f326d98ce7906205f0ce41c2e7)

The Linode COSI Driver is an implementation of the Kubernetes [Container Object Storage Interface (COSI)](https://github.com/kubernetes-sigs/container-object-storage-interface) standard. COSI provides a consistent and unified way to expose object storage to containerized workloads running in Kubernetes. This driver specifically enables integration with Linode Object Storage service, making it easier for Kubernetes applications to interact with Linode's scalable and reliable object storage infrastructure.

- [Linode COSI Driver](#linode-cosi-driver)
  - [Getting Started](#getting-started)
  - [Configuration](#configuration)
    - [BucketClass](#bucketclass)
    - [BucketAccessClass](#bucketaccessclass)
  - [Testing](#testing)
  - [License](#license)
  - [Support](#support)
  - [Contributing](#contributing)

## Getting Started

Follow these steps to get started with Linode COSI Driver:

1. **Prerequisites:**
    1. Install the matching COSI CRDs and controller.

    ```sh
    just deploy-deps
    ```

2. **Installation:**
    1. Create new API token in [Akamai Cloud Manager](https://cloud.linode.com/profile/tokens). The token must be configured with the following permissions:
        - **Object Storage** - Read/Write

    2. Build a driver image from this checkout and install the local Helm chart.
       The released v1alpha1 driver image cannot communicate with the v1alpha2 sidecar.

    ```sh
    # For a local KIND development environment, build and deploy with Tilt:
    just local-deploy

    # For another development cluster, publish this checkout's driver image first,
    # then explicitly select that image when installing the local chart:
    helm upgrade --install cosi ./helm/linode-cosi-driver \
        --set=apiToken=<YOUR_LINODE_API_TOKEN> \
        --set=driver.image.repository=<YOUR_DRIVER_IMAGE_REPOSITORY> \
        --set=driver.image.tag=<YOUR_V1ALPHA2_DRIVER_TAG> \
        --namespace=linode-cosi-driver \
        --create-namespace
    ```

3. **Usage:**
    1. Create Bucket Class (see the [example.BucketClass.yaml](./examples/linode-objectstorage.BucketClass.yaml)).

    ```sh
    kubectl create -f ./examples/linode-objectstorage.BucketClass.yaml
    ```

    2. Create Bucket Access Class (see the [example.BucketAccessClass.yaml](./examples/linode-objectstorage.BucketAccessClass.yaml)).

    ```sh
    kubectl create -f ./examples/linode-objectstorage.BucketAccessClass.yaml
    ```

    3. Create Bucket Claim (see the [example.BucketClaim.yaml](./examples/example.BucketClaim.yaml)).

    ```sh
    kubectl create -f ./examples/example.BucketClaim.yaml
    ```

    4. Create Bucket Access (see the [example.BucketAccess.yaml](./examples/example.BucketAccess.yaml)).

    ```sh
    kubectl create -f ./examples/example.BucketAccess.yaml
    ```

    5. Consume the `s3-credentials` Secret as environment variables:

    ```yaml
    spec:
      containers:
        - name: application
          envFrom:
            - secretRef:
                name: s3-credentials
    ```

    The v1alpha2 Secret contains `COSI_S3_BUCKET_ID`,
    `COSI_S3_ENDPOINT`, `COSI_S3_REGION`, `COSI_S3_ADDRESSING_STYLE`,
    `COSI_S3_ACCESS_KEY_ID`, and `COSI_S3_ACCESS_SECRET_KEY`.

    AWS SDK/CLI applications must map these entries to their own configuration
    variables, as shown in the sample application end-to-end test.

## v1alpha2 development baseline

This branch uses unmodified upstream COSI commit
`7b121df3e445b9f326d98ce7906205f0ce41c2e7` and its published controller/sidecar
image tag `v20261009-controllerv0.2.0-rc1-348-g7b121df`. The latest tagged COSI
release, `v0.2.2`, serves v1alpha1 and cannot be used with this driver.

The driver implements only the v1alpha2 gRPC service. Deploy matching CRDs,
controller, sidecar, and application resources together in a fresh development
cluster. This is not an in-place conversion of existing v1alpha1 resources.

Supported access is one bucket, S3, and Key authentication. ReadOnly and ReadWrite
modes are supported. Multi-bucket access returns `OutOfRange`; WriteOnly,
ServiceAccount authentication, other protocols, and incompatible permissions
across data/metadata categories return `InvalidArgument`.

Bucket identity generation is deterministic and does not create backend
resources. Revocation uses the recorded account ID and does not require a live
BucketClaim or Bucket object. Bucket deletion preserves the existing forced
cleanup option.

Credential creation retains an existing driver limitation: Linode returns the
secret key only when creating the key. Repeated grant calls can create additional
keys; restart-safe grant retries require a durable credential store. This needs
separate work before treating the driver as production-ready.

## Configuration

Here’s the updated table with descriptions added:

### BucketClass

| Parameter                   | Default    | Values                                                                                               | Description                                                                            |
|-----------------------------|------------|------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------------------|
| `cosi.linode.com/v1/region` |            | <https://techdocs.akamai.com/linode-api/reference/get-object-storage-endpoints>                        | **REQUIRED** The region where the object storage bucket will be created.               |
| `cosi.linode.com/v1/acl`    | `private`  | `private`, `public-read`, `authenticated-read`, `public-read-write`                                  | The access control list (ACL) policy that defines who can read or write to the bucket. |
| `cosi.linode.com/v1/cors`   | `disabled` | `disabled`, `enabled`                                                                                | Enables or disables Cross-Origin Resource Sharing (CORS) for the bucket.               |
| `cosi.linode.com/v1/cleanup` |            | `force`                                                                                              | Deletes all objects before deleting the bucket. If omitted, deletion of a non-empty bucket fails. |
| `cosi.linode.com/v1/endpoint-type` | first available | `E0`, `E1`, `E2`, `E3`                                                                       | Selects the Object Storage endpoint type used when creating the bucket.                |
| `cosi.linode.com/v1/endpoint-type-preference` | first available | Comma-separated `E0`, `E1`, `E2`, `E3` values, for example `E3,E1`                 | Selects the first available Object Storage endpoint type for the bucket in preference order. Ignored when `endpoint-type` is set. |
| `cosi.linode.com/v1/policy` |            | <https://techdocs.akamai.com/cloud-computing/docs/define-access-and-permissions-using-bucket-policies> | Defines custom bucket policies for fine-grained access control and permissions. Policies granting a wildcard principal require the `public-read` or `public-read-write` ACL. |

### BucketAccessClass

| Parameter                        | Default     | Values                    | Description                                                                                                             |
|----------------------------------|-------------|---------------------------|-------------------------------------------------------------------------------------------------------------------------|
| `cosi.linode.com/v1/endpoint-type` | first available | `E0`, `E1`, `E2`, `E3` | Selects which Object Storage endpoint type to return in generated bucket credentials.                                   |
| `cosi.linode.com/v1/endpoint-type-preference` | first available | Comma-separated `E0`, `E1`, `E2`, `E3` values, for example `E3,E1` | Selects the first available Object Storage endpoint type for generated bucket credentials in preference order. Ignored when `endpoint-type` is set. |
Request ReadOnly or ReadWrite permissions through the BucketAccess
`spec.bucketClaims[].accessModes` fields. The legacy
`cosi.linode.com/v1/permissions` class parameter is no longer used.

## Testing

Run `just test` for unit tests and `just test-integration` for backend integration
tests. `just test-e2e` deploys the local KIND environment and runs Chainsaw.
Backend tests require `LINODE_TOKEN`; when targeting a non-default API, set
`LINODE_URL` and `LINODE_API_VERSION` consistently for the driver and test runner.

The generic COSI end-to-end test covers credentials Secret deletion before
BucketAccess deletion. It verifies that the Secret is pending deletion with its
protection finalizer, then checks that access cleanup revokes the backend key and
removes both Kubernetes resources while leaving the BucketClaim ready. It checks
the key exists before deletion so an incorrect account/API target cannot produce
a false positive. No namespace deletion or manual finalizer removal is used to
complete this test.

## License

Linode COSI Driver is licensed under the [Apache 2.0](LICENSE) terms. Please review it before using or contributing to the project.

## Support

For any issues, questions, or support, please [create an issue](https://github.com/linode/linode-cosi-driver/issues).

## Contributing

We welcome contributions! If you have ideas, bug reports, or want to contribute code, please check out our [Contribution Guidelines](CONTRIBUTING.md).
