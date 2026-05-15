# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Added CONTRIBUTING.md with development instructions
- Add Grafana dashboards for Strimzi Cruise control and Strimzi operator

### Changed

- Rework README.md: more installation methods, enriched Kafka cluster creation example, reworded configuration options/Helm values, moved CRDs management section, removed examples manifests.

## [0.1.1] - 2026-05-04

### Fixed

- Fix additionalProperties for Helm value schema

## [0.1.0] - 2026-05-04

### Added

- Add pre-commit setup

### Changed

- Upgrade to Strimzi Kafka Operator v1.0.0, which moves CRDs to v1 while dropping support for beta versions, and various bug fixes and improvements.
- Vendor the upstream chart directly into this repo to provide seamless CRDs installation and upgrades by patching the upstream chart and removeing the CRDs from it.
- Update Helm values description and annotation

### Fixed

- Fix deprecated Auto updateMode for verticalPodAutoscaler, replace it with Recreate

## [0.0.2] - 2026-04-01

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

[Unreleased]: https://github.com/giantswarm/strimzi-kafka-operator/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/giantswarm/strimzi-kafka-operator/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/giantswarm/strimzi-kafka-operator/compare/v0.0.2...v0.1.0
[0.0.2]: https://github.com/giantswarm/strimzi-kafka-operator/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/giantswarm/strimzi-kafka-operator/releases/tag/v0.0.1
