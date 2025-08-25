k8s_yaml(kustomize("./hack/container-object-storage-controller"))
k8s_resource(
    workload="container-object-storage-controller",
    objects=[
        "container-object-storage-system:namespace",
        "bucketaccessclasses.objectstorage.k8s.io:customresourcedefinition",
        "bucketaccesses.objectstorage.k8s.io:customresourcedefinition",
        "bucketclaims.objectstorage.k8s.io:customresourcedefinition",
        "bucketclasses.objectstorage.k8s.io:customresourcedefinition",
        "buckets.objectstorage.k8s.io:customresourcedefinition",
        "container-object-storage-controller-sa:serviceaccount",
        "container-object-storage-controller:role",
        "container-object-storage-controller-role:clusterrole",
        "container-object-storage-controller:rolebinding",
        "container-object-storage-controller:clusterrolebinding",
])
k8s_yaml(helm( "./helm/linode-cosi-driver",
    "linode-cosi-driver",
    namespace="linode-cosi-driver",
    set=[
        "apiToken=" + os.getenv("LINODE_TOKEN"),
    ],
))

k8s_resource(
    workload="linode-cosi-driver",
    objects=[
        "linode-cosi-driver:serviceaccount",
        "linode-cosi-driver:clusterrole",
        "linode-cosi-driver:clusterrolebinding",
        "linode-cosi-driver:secret",
    ],
)
if os.getenv("SKIP_DOCKER_BUILD", "false") != "true":
    docker_build(
        "docker.io/linode/linode-cosi-driver",
        context=".",
        only=("Dockerfile", "Makefile", "vendor", "go.mod", "go.sum", "./pkg", "./cmd"),
    )
