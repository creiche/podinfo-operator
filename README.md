# podinfo-operator
Operator to setup and manage podinfo with a redis backend

## Getting Started

### Running unit tests

Unit tests have been created to test the functions of the controller using envtest without the need for a cluster:

```sh
make test
```

### Running locally on your laptop
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

1. Install CRDs:

```sh
make install
```

2. Run the controller locally on your laptop:

```sh
make run
```

3. Create sample resource:

```sh
kubectl apply -f config/samples/
```

Sample resource created:
```yaml
apiVersion: my.api.group/v1alpha1
kind: MyAppResource
metadata:
  labels:
    app.kubernetes.io/name: myappresource
    app.kubernetes.io/instance: myappresource-sample
    app.kubernetes.io/part-of: podinfo-operator
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: podinfo-operator
  name: myappresource-sample
spec:
  replicaCount: 2
  resources:
    memoryLimit: 64Mi
    cpuRequest: 100m
  image:
    repository: ghcr.io/stefanprodan/podinfo
    tag: latest
  ui:
    color: "#34577c"
    message: "some string"
  redis:
    enabled: true
```

### Deploying Controller to the cluster
You will need to push the docker image to somewhere that your local cluster can pull from.

1. Build and Push Image:

```sh
make docker-build docker-push IMG=<some-registry>/podinfo-operator:tag
```

2. Deploy controller to cluster:

```sh
make deploy IMG=<some-registry>/podinfo-operator:tag
```

### Testing Application

1. Setup port forwarding to the Podinfo service:

```sh
kubectl port-forward svc/myappresource-sample-podinfo 8888:80
```

2. Open a browser to `http://localhost:8888/`

3. Post data to test Redis connectivity

```sh
curl -X POST localhost:8888/cache/{key} --data {data}
```

4. Get data back to ensure Redis connectivity

```sh
curl localhost:8888/cache/{key}
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller from the cluster:

```sh
make undeploy
```
