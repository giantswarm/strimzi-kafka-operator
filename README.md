[![CircleCI](https://dl.circleci.com/status-badge/img/gh/giantswarm/strimzi-kafka-operator/tree/main.svg?style=svg)](https://dl.circleci.com/status-badge/redirect/gh/giantswarm/strimzi-kafka-operator/tree/main)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/giantswarm/strimzi-kafka-operator/badge)](https://securityscorecards.dev/viewer/?uri=github.com/giantswarm/strimzi-kafka-operator)

# strimzi-kafka-operator app

Giant Swarm app wrapping the [Strimzi Kafka Operator](https://strimzi.io/), which manages
Apache Kafka clusters natively on Kubernetes via custom resources (CRDs).

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

## CRD management

CRDs are managed by this chart's `templates/crds/` (updated on `helm upgrade`, kept on
`helm uninstall`). **Flux and Argo CD users:** the bundled subchart also ships CRDs and
needs `--skip-crds` equivalents — see the `crds:` block in
[helm/strimzi-kafka-operator/values.yaml](helm/strimzi-kafka-operator/values.yaml) for the
full lifecycle, rationale, and per-tool flags.

### After a version bump (Renovate PR)

When Renovate bumps the chart version in `Chart.yaml`, re-sync the upstream chart:

```bash
make sync-chart
git add helm/strimzi-kafka-operator/
git commit -m "Sync upstream chart for strimzi-kafka-operator v<new-version>"
```

## Compatibility

| App version | Upstream chart | Kubernetes |
|---|---|---|
| 0.1.x | strimzi-kafka-operator 1.0.0 | 1.30+ |

## Development

```bash
# Update upstream chart dependency and CRDs
make sync-chart

# Lint
helm lint helm/strimzi-kafka-operator --values helm/strimzi-kafka-operator/ci/default-values.yaml

# Template render (dry-run)
helm template strimzi-kafka-operator helm/strimzi-kafka-operator \
  --values helm/strimzi-kafka-operator/ci/default-values.yaml --debug
```

## Creating and testing a Kafka cluster

Start with the upstream docs:

- [Strimzi Quickstart](https://strimzi.io/quickstarts/) — create a cluster, send and receive messages
- [Strimzi overview](https://strimzi.io/docs/operators/latest/overview)
- [Example Kafka resources](https://github.com/strimzi/strimzi-kafka-operator/tree/0.51.0/examples/kafka)

> **Giant Swarm note:** the hardened pod security policies on GS clusters block the
> `kubectl run` commands in the upstream quickstart. Instead, exec into a broker pod
> and run the producer/consumer scripts from there:
>
> ```bash
> kubectl exec -ti my-cluster-broker-0 -- /bin/sh
> # then: bin/kafka-console-producer.sh, bin/kafka-topics.sh, ...
> ```

## References

- Changelog: [CHANGELOG.md](CHANGELOG.md)
- Security policy: [SECURITY.md](SECURITY.md)
- Upstream values reference: [strimzi-kafka-operator on ArtifactHub](https://artifacthub.io/packages/helm/strimzi/strimzi-kafka-operator)
- Upstream source: [strimzi/strimzi-kafka-operator](https://github.com/strimzi/strimzi-kafka-operator)
- Upstream Helm chart: [strimzi-kafka-operator chart](https://github.com/strimzi/strimzi-kafka-operator/tree/main/helm-charts/helm3/strimzi-kafka-operator)
