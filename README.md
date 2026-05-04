[![CircleCI](https://dl.circleci.com/status-badge/img/gh/giantswarm/strimzi-kafka-operator/tree/main.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/giantswarm/strimzi-kafka-operator/tree/main)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/giantswarm/strimzi-kafka-operator/badge)](https://securityscorecards.dev/viewer/?uri=github.com/giantswarm/strimzi-kafka-operator)

# strimzi-kafka-operator app

Giant Swarm app wrapping the [Strimzi Kafka Operator](https://strimzi.io/), which manages
Apache Kafka clusters natively on Kubernetes via custom resources (CRDs).

## Quick Start

### Install strimzi-kafka-operator

#### Install via Giant Swarm App plaform CLI

This install method requires [`kubectl gs`](https://docs.giantswarm.io/reference/kubectl-gs/installation/) CLI and also make sure you are logged into a **management cluster** using `kubectl gs login`.

Then run the following command to deploy the app to your cluster. This will create the necessary `OCIRepository` and `HelmRelease` resources for Flux to deploy the app:

```shell
$ kubectl gs deploy chart \
    --chart-name strimzi-kafka-operator \
    --version 0.1.1 \
    --organization demo \
    --target-cluster test \
    --target-namespace kafka
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  labels:
    giantswarm.io/cluster: test
  name: test-strimzi-kafka-operator
  namespace: org-demo
spec:
  interval: 10m0s
  provider: generic
  ref:
    tag: 0.1.1
  url: oci://gsoci.azurecr.io/charts/giantswarm/strimzi-kafka-operator
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  labels:
    giantswarm.io/cluster: test
  name: test-strimzi-kafka-operator
  namespace: org-demo
spec:
  chartRef:
    kind: OCIRepository
    name: test-strimzi-kafka-operator
  install:
    createNamespace: true
  interval: 10m0s
  kubeConfig:
    secretRef:
      name: test-kubeconfig
  releaseName: strimzi-kafka-operator
  targetNamespace: kafka
Applied OCIRepository org-demo/test-strimzi-kafka-operator
Applied HelmRelease org-demo/test-strimzi-kafka-operator

$ kubectl -n org-demo wait --for=condition=Ready helmrelease/test-strimzi-kafka-operator
helmrelease.helm.toolkit.fluxcd.io/test-strimzi-kafka-operator condition met
```

#### Install via GitOps on a Giant Swarm cluster

See [Adding an App via GitOps](https://docs.giantswarm.io/tutorials/continuous-deployment/apps/add-appcr/).

#### Install via Helm

This method requires [`Helm`](https://helm.sh/docs/intro/install/) and also make sure you are logged into a **workload cluster** using `kubectl gs login`.

Then run the following commands to add the chart repository and install the chart:

```
$ helm repo add giantswarm https://giantswarm.github.io/giantswarm-catalog/
"giantswarm" has been added to your repositories
$ helm repo update
Hang tight while we grab the latest from your chart repositories...
...Successfully got an update from the "giantswarm" chart repository
Update Complete. ⎈Happy Helming!⎈
$ helm install strimzi giantswarm/strimzi-kafka-operator
NAME: strimzi
LAST DEPLOYED: Mon May  4 17:34:03 2026
NAMESPACE: kafka
STATUS: deployed
REVISION: 1
DESCRIPTION: Install complete
TEST SUITE: None
NOTES:
Strimzi Kafka Operator 1.0.0 is now running in kube-system namespace.
It watches for Kafka custom resources in: all namespaces.

To create a Kafka cluster refer to the following documentation.

https://strimzi.io/docs/operators/latest/deploying#deploying-kafka-cluster-kraft-str

For full documentation see:
  https://github.com/giantswarm/strimzi-kafka-operator
  https://strimzi.io/documentation/
```

## Create a Kafka cluster

Make sure you are logged into the cluster where the operator is running.

Then run the following commands to create a Kafka cluster:

```
$ kubectl apply -f examples/kafka-ephemeral.yaml
kafkanodepool.kafka.strimzi.io/controller created
kafkanodepool.kafka.strimzi.io/broker created
kafka.kafka.strimzi.io/my-cluster created
$ kubectl wait kafka/my-cluster --for=condition=Ready --timeout=10m
kafka.kafka.strimzi.io/my-cluster condition met
```

You can now test your cluster using the [official Kafka Quickstart](https://kafka.apache.org/quickstart/#step-3-create-a-topic-to-store-your-events) in order to create a topic and sending + reading messages. In order to run the CLI tools from the official Kafka Quickstart, you can exec into a broker pod as follow:

```bash
kubectl exec -ti my-cluster-broker-0 -- /bin/sh
# then: bin/kafka-console-producer.sh, bin/kafka-topics.sh, ...
```

**Giant Swarm note:** the hardened pod security policies on GS clusters block the
`kubectl run` commands in the upstream quickstart. Instead, exec into a broker pod
and run the producer/consumer scripts from there.

Upstream documentation:

- [Strimzi Quickstart](https://strimzi.io/quickstarts/) — create a cluster, send and receive messages
- [Strimzi overview](https://strimzi.io/docs/operators/latest/overview)
- [Example Kafka resources](https://github.com/strimzi/strimzi-kafka-operator/tree/1.0.0/examples/kafka)

## Configuring

Here are some configuration options to consider when configuring your Helm chart values:

```yaml
# CRD management (disable if you manage CRDs externally)
crds:
  install: true

# Upstream overrides (see full list in values.yaml)
strimzi-kafka-operator:
  # Default: true (watches all namespaces — recommended for a platform operator).
  # Set to false and use watchNamespaces to restrict to specific namespaces.
  watchAnyNamespace: true
  replicas: 2
```

See [helm/strimzi-kafka-operator/values.yaml](helm/strimzi-kafka-operator/values.yaml) for all options.

## CRD management

CRDs are managed by this chart's `templates/crds/` (updated on `helm upgrade`, kept on
`helm uninstall`). See the `crds:` block in
[helm/strimzi-kafka-operator/values.yaml](helm/strimzi-kafka-operator/values.yaml) for the
full lifecycle.

## Images

All images are sourced from `gsoci.azurecr.io` (retagged from upstream `quay.io/strimzi`).

| Image | Tag |
|---|---|
| `gsoci.azurecr.io/giantswarm/strimzi/operator` | `1.0.0` |
| `gsoci.azurecr.io/giantswarm/strimzi/kafka` | `1.0.0-kafka-<ver>` |
| `gsoci.azurecr.io/giantswarm/strimzi/kafka-bridge` | `1.0.0` |
| `gsoci.azurecr.io/giantswarm/strimzi/kaniko-executor` | `1.0.0` |
| `gsoci.azurecr.io/giantswarm/strimzi/buildah` | `1.0.0` |
| `gsoci.azurecr.io/giantswarm/strimzi/maven-builder` | `1.0.0` |

## Compatibility

| App version | Upstream chart | Kubernetes |
|---|---|---|
| 0.1.x | strimzi-kafka-operator 1.0.0 | 1.30+ |

## References

- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- Security policy: [SECURITY.md](SECURITY.md)
- Upstream values reference: [strimzi-kafka-operator on ArtifactHub](https://artifacthub.io/packages/helm/strimzi/strimzi-kafka-operator)
- Upstream source: [strimzi/strimzi-kafka-operator](https://github.com/strimzi/strimzi-kafka-operator)
- Upstream Helm chart: [strimzi-kafka-operator chart](https://github.com/strimzi/strimzi-kafka-operator/tree/main/helm-charts/helm3/strimzi-kafka-operator)
