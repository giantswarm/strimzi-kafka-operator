# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- E2E tests for workload cluster deployments: operator readiness, Kafka/KafkaNodePool CR lifecycle, broker pod readiness, entity-operator readiness, and metrics availability in Mimir.

## [0.0.1] - 2026-03-30

### Added

- Initial release wrapping upstream `strimzi-kafka-operator` chart v0.51.0.
- CRD installation and **upgrade** support via `templates/crds/` (not Helm's `crds/` dir),
  with `helm.sh/resource-policy: keep` to prevent data loss on uninstall.
  Run `make sync-crds` after bumping the chart version in `Chart.yaml`.
- GS-specific NetworkPolicy support (`networkPolicy.flavor: cilium | kubernetes`).
- CiliumNetworkPolicy for CAPI clusters with Cilium CNI.
- Standard Kubernetes NetworkPolicy for non-Cilium clusters.
- KyvernoPolicyException to exempt the operator pod from restrictive GS policies.
- Hardened pod/container security contexts (non-root, drop ALL capabilities, seccomp RuntimeDefault).
- All images sourced from `gsoci.azurecr.io/giantswarm/strimzi/*` (retagged from upstream `quay.io/strimzi/*`).
- Helm CI test values under `helm/strimzi-kafka-operator/ci/default-values.yaml`.
- `make sync-crds` Makefile target to re-extract CRDs after upstream version bumps.
- `make show-images` Makefile target listing all images that require retagging.
- Renovate `customManager` to track `kafka-bridge` tag independently from the operator version.
- Renovate `postUpgradeTasks` to run `make sync-crds` automatically after a chart version bump.
- PodDisruptionBudget enabled via upstream chart's built-in support (`podDisruptionBudget.enabled: true`, `minAvailable: 1`).
- TopologySpreadConstraints to spread operator pods across nodes (`whenUnsatisfiable: ScheduleAnyway`).
- VerticalPodAutoscaler template (`verticalPodAutoscaler.enabled`, disabled by default; requires VPA CRDs on the cluster).

### Images (strimzi-kafka-operator v0.51.0, Kafka 4.x)

All images retagged from `quay.io/strimzi/*` → `gsoci.azurecr.io/giantswarm/strimzi/*`.

| Source image | Tag | Notes |
|---|---|---|
| `quay.io/strimzi/operator` | `0.51.0` | Operator, topic-operator, user-operator, kafka-init |
| `quay.io/strimzi/kafka` | `0.51.0-kafka-4.1.0` | Kafka broker, connect, mirror-maker-2, cruise-control, exporter |
| `quay.io/strimzi/kafka` | `0.51.0-kafka-4.1.1` | Same components, Kafka 4.1.1 |
| `quay.io/strimzi/kafka` | `0.51.0-kafka-4.2.0` | Same components, Kafka 4.2.0 |
| `quay.io/strimzi/kafka-bridge` | `0.33.1` | Independent release cadence; tracked by Renovate |
| `quay.io/strimzi/kaniko-executor` | `0.51.0` | Connector build support |
| `quay.io/strimzi/buildah` | `0.51.0` | Connector build support |
| `quay.io/strimzi/maven-builder` | `0.51.0` | Connector build support |

[Unreleased]: https://github.com/giantswarm/strimzi-kafka-operator/compare/v0.0.1...HEAD
[0.0.1]: https://github.com/giantswarm/strimzi-kafka-operator/releases/tag/v0.0.1
