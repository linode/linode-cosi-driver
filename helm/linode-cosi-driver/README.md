# linode-cosi-driver

![Version: 0.2.1](https://img.shields.io/badge/Version-0.2.1-informational?style=flat) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat) ![AppVersion: 0.4.0](https://img.shields.io/badge/AppVersion-0.4.0-informational?style=flat)

A Helm chart for Kubernetes

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` | Node affinity rules for pod assignment. |
| apiToken | string | `""` | Linode API token. This field is **required** unless secret is created before deployment (see `secret.ref` value). |
| driver.image.pullPolicy | string | `"IfNotPresent"` | Driver container image pull policy. |
| driver.image.repository | string | `"docker.io/linode/linode-cosi-driver"` | Driver container image repository. |
| driver.image.tag | string | `""` | Overrides the image tag whose default is the chart appVersion. |
| driver.otelConfig | object | `{}` | OpenTelemetry configuration. All values defined here conform to the OTEL specification, and are not strictly defined in the Chart values. |
| fullnameOverride | string | `""` | Overrides the full chart name. |
| imagePullSecrets | list | `[]` | List of Docker registry secret names to pull images. |
| linodeApiUrl | string | `""` | Linode API URL, leave empty for default. |
| linodeApiVersion | string | `""` | Linode API version, leave empty for default. |
| nameOverride | string | `""` | Overrides the chart name. |
| nodeSelector | object | `{}` | Node labels for pod assignment. |
| podAnnotations | object | `{"prometheus.io/path":"/metrics","prometheus.io/port":"9464","prometheus.io/scrape":"true"}` | Annotations to add to the pod. |
| podSecurityContext.runAsNonRoot | bool | `true` | Run the pod as a non-root user. |
| podSecurityContext.runAsUser | int | `65532` | User ID to run the pod as. |
| rbac.annotations | object | `{}` | Annotations to add to the service account, cluster role, and cluster role binding. |
| rbac.name | string | `""` | The name of the service account, cluster role, and cluster role binding to use. If not set, a name is generated using the fullname template. |
| replicaCount | int | `1` | Number of pod replicas. |
| resources | object | `{}` | Specify CPU and memory resource limits if needed. The value defined for CPU limits affects the number of threads used in the driver. The number of CPU seconds allocated above 1 is rounded using floor operation, so it should be done in integer steps (e.g. from 1 to 2). This means that assigning CPU limit of 1.5 will result in only one CPU being used at a time. |
| secret.annotations | object | `{}` | Annotations to add to the secret. |
| secret.ref | string | `""` | Name of existing secret. If not set, a new secret is created. |
| securityContext.readOnlyRootFilesystem | bool | `true` | Container runs with a read-only root filesystem. |
| sidecar.image.pullPolicy | string | `"IfNotPresent"` | Sidecar container image pull policy. |
| sidecar.image.repository | string | `"gcr.io/k8s-staging-sig-storage/objectstorage-sidecar/objectstorage-sidecar"` | Sidecar container image repository. |
| sidecar.image.tag | string | `"v20221117-v0.1.0-22-g0e67387"` | Sidecar container image tag. |
| sidecar.logVerbosity | int | `4` | Log verbosity level for the sidecar container. |
| tolerations | list | `[]` | Tolerations for pod assignment. |

