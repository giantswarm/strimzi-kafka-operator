[![CircleCI](https://dl.circleci.com/status-badge/img/gh/giantswarm/strimzi-kafka-operator/tree/main.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/giantswarm/strimzi-kafka-operator/tree/main)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/giantswarm/strimzi-kafka-operator/badge)](https://securityscorecards.dev/viewer/?uri=github.com/giantswarm/strimzi-kafka-operator)

# strimzi-kafka-operator app

Giant Swarm app wrapping the [Strimzi Kafka Operator](https://strimzi.io/), which manages
Apache Kafka clusters natively on Kubernetes via custom resources (CRDs).

**What is this app?**
Strimzi simplifies running Kafka on Kubernetes by providing a Kubernetes-native way to
deploy and manage Kafka clusters, topics, users, and connectors through CRDs.

**Who can use it?**
Teams that need to run Apache Kafka on Giant Swarm workload clusters.

## Installing

Deploy via the Giant Swarm App Platform:

```yaml
apiVersion: application.giantswarm.io/v1alpha1
kind: App
metadata:
  name: strimzi-kafka-operator
  namespace: <cluster-id>
spec:
  name: strimzi-kafka-operator
  namespace: strimzi-system
  version: 0.1.0
  catalog: giantswarm
```

Or using GitOps — see [Adding an App via GitOps](https://docs.giantswarm.io/tutorials/continuous-deployment/apps/add-appcr/).

## Configuring

### Key values

```yaml
# CRD management (disable if you manage CRDs externally)
crds:
  install: true

# NetworkPolicy flavor — matches your cluster's CNI
networkPolicy:
  enabled: true
  flavor: cilium        # "cilium" | "kubernetes"

# Kyverno policy exceptions
kyvernoPolicyExceptions:
  enabled: true
  namespace: giantswarm

# Upstream overrides (see full list in values.yaml)
strimzi-kafka-operator:
  # Default: true (watches all namespaces — recommended for a platform operator).
  # Set to false and use watchNamespaces to restrict to specific namespaces.
  watchAnyNamespace: true
  replicas: 2
```

See [helm/strimzi-kafka-operator/values.yaml](helm/strimzi-kafka-operator/values.yaml) for all options.

### Sample App CR

```yaml
apiVersion: application.giantswarm.io/v1alpha1
kind: App
metadata:
  name: strimzi-kafka-operator
  namespace: <cluster-id>
spec:
  name: strimzi-kafka-operator
  namespace: strimzi-system
  version: 0.1.0
  catalog: giantswarm
  userConfig:
    configMap:
      name: strimzi-kafka-operator-user-values
      namespace: <cluster-id>
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: strimzi-kafka-operator-user-values
  namespace: <cluster-id>
data:
  values: |
    networkPolicy:
      enabled: true
      flavor: cilium
    strimzi-kafka-operator:
      watchAnyNamespace: true
```

## Images

All images are sourced from `gsoci.azurecr.io` (retagged from upstream `quay.io/strimzi`).

| Image | Tag |
|---|---|
| `gsoci.azurecr.io/giantswarm/strimzi/operator` | `0.51.0` |
| `gsoci.azurecr.io/giantswarm/strimzi/kafka` | `0.51.0-kafka-<ver>` |
| `gsoci.azurecr.io/giantswarm/strimzi/kafka-bridge` | `0.33.1` |
| `gsoci.azurecr.io/giantswarm/strimzi/kaniko-executor` | `0.51.0` |
| `gsoci.azurecr.io/giantswarm/strimzi/buildah` | `0.51.0` |
| `gsoci.azurecr.io/giantswarm/strimzi/maven-builder` | `0.51.0` |

## CRD management

CRDs are placed in `helm/strimzi-kafka-operator/templates/crds/` (not Helm's `crds/` directory).
This means CRDs are **updated on `helm upgrade`** (Helm's `crds/` dir only installs, never upgrades).

The annotation `helm.sh/resource-policy: keep` prevents CRD deletion on `helm uninstall`,
protecting any existing Kafka CR data.

### After a version bump (Renovate PR)

When Renovate bumps the chart version in `Chart.yaml`, re-extract the CRDs:

```bash
make sync-crds
git add helm/strimzi-kafka-operator/templates/crds/
git commit -m "Sync CRDs for strimzi-kafka-operator v<new-version>"
```

## Compatibility

| App version | Upstream chart | Kubernetes |
|---|---|---|
| 0.1.x | strimzi-kafka-operator 0.51.0 | 1.27+ |

## Development

```bash
# Update upstream chart dependency
helm dep update helm/strimzi-kafka-operator

# Re-extract CRDs after version bump
make sync-crds

# Lint
helm lint helm/strimzi-kafka-operator --values helm/strimzi-kafka-operator/ci/default-values.yaml

# Template render (dry-run)
helm template strimzi-kafka-operator helm/strimzi-kafka-operator \
  --values helm/strimzi-kafka-operator/ci/default-values.yaml --debug
```

## Credit

- Upstream: [strimzi/strimzi-kafka-operator](https://github.com/strimzi/strimzi-kafka-operator)
- Helm chart: [Strimzi Helm charts](https://github.com/strimzi/strimzi-kafka-operator/tree/main/helm-charts/helm3/strimzi-kafka-operator)
- ArtifactHub: [strimzi-kafka-operator](https://artifacthub.io/packages/helm/strimzi/strimzi-kafka-operator)
