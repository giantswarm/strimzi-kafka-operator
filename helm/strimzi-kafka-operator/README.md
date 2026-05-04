# strimzi-kafka-operator

![Version: 0.0.2](https://img.shields.io/badge/Version-0.0.2-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 1.0.0](https://img.shields.io/badge/AppVersion-1.0.0-informational?style=flat-square)

Giant Swarm app wrapping the Strimzi Kafka Operator, which manages Apache Kafka clusters natively on Kubernetes via custom resources.

**Homepage:** <https://github.com/giantswarm/strimzi-kafka-operator>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| giantswarm/team-atlas | <team-atlas@giantswarm.io> |  |

## Source Code

* <https://github.com/strimzi/strimzi-kafka-operator>
* <https://github.com/giantswarm/strimzi-kafka-operator>

## Requirements

| Repository | Name | Version |
|------------|------|---------|
|  | strimzi-kafka-operator | 1.0.0 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| crds | object | `{"install":true}` | CRD lifecycle management. CRDs are placed in templates/crds/ (not helm's crds/ dir) so they are updated on `helm upgrade`. The `helm.sh/resource-policy: keep` annotation prevents deletion on `helm uninstall` to protect user data. |
| verticalPodAutoscaler | object | `{"enabled":true,"updateMode":"Recreate"}` | VerticalPodAutoscaler for the operator Deployment. Requires the VPA CRD to be installed on the cluster (available on most GS CAPI clusters). Disabled by default; enable per-installation if VPA is available. |
| podMonitor | object | `{"enabled":true,"labels":{"observability.giantswarm.io/tenant":"giantswarm"},"workloads":{"enabled":true}}` | PodMonitor for scraping operator metrics (port 8080 / Prometheus format). Docs: https://docs.giantswarm.io/overview/observability/data-management/data-ingestion/ |
| podMonitor.workloads | object | `{"enabled":true}` | PodMonitors for Kafka workloads managed by the operator (cross-namespace). |
| dashboards | object | `{"annotations":{"observability.giantswarm.io/folder":"Kafka","observability.giantswarm.io/organization":"Shared Org"},"enabled":true,"label":"app.giantswarm.io/kind","labelValue":"dashboard","namespace":""}` | Grafana dashboards (9 dashboards: Kafka, KRaft, KafkaConnect, MirrorMaker2, Bridge, CruiseControl, Exporter, OAuth, Operators). The upstream chart's dashboard template is disabled because the JSON files lack "uid" fields required by the GS admission webhook. This wrapper manages the ConfigMaps instead, using JSONs extracted and UID-patched via `make sync-dashboards`. Docs: https://docs.giantswarm.io/overview/observability/dashboard-management/dashboard-creation/ |
| strimzi-kafka-operator | object | `{"affinity":{"podAntiAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"podAffinityTerm":{"labelSelector":{"matchLabels":{"name":"strimzi-cluster-operator","strimzi.io/kind":"cluster-operator"}},"topologyKey":"kubernetes.io/hostname"},"weight":100}]}},"dashboards":{"annotations":{"observability.giantswarm.io/folder":"Kafka","observability.giantswarm.io/organization":"Shared Org"},"enabled":false,"label":"app.giantswarm.io/kind","labelValue":"dashboard"},"defaultImageRegistry":"gsoci.azurecr.io","defaultImageRepository":"giantswarm/strimzi","enabled":true,"extraEnvs":[{"name":"STRIMZI_POD_SECURITY_PROVIDER_CLASS","value":"io.strimzi.plugin.security.profiles.impl.RestrictedPodSecurityProvider"}],"generateNetworkPolicy":true,"kafkaBridge":{"image":{"name":"kafka-bridge","registry":"","repository":"","tag":"1.0.0"}},"labels":{"observability.giantswarm.io/tenant":"giantswarm"},"leaderElection":{"enable":true},"operatorNetworkPolicy":{"egress":[{}],"enabled":true},"podDisruptionBudget":{"enabled":true,"minAvailable":1},"podSecurityContext":{"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}},"replicas":2,"resources":{"limits":{"cpu":"1000m","memory":"384Mi"},"requests":{"cpu":"200m","memory":"384Mi"}},"securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true,"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}},"topologySpreadConstraints":[{"labelSelector":{"matchLabels":{"name":"strimzi-cluster-operator"}},"maxSkew":1,"topologyKey":"kubernetes.io/hostname","whenUnsatisfiable":"ScheduleAnyway"}],"watchAnyNamespace":true}` | --------------------------------------------------------------------- |
| strimzi-kafka-operator.defaultImageRegistry | string | `"gsoci.azurecr.io"` | Global image registry/repository defaults. All component images fall back to these when their per-component registry/repository fields are empty.  Images retagged from quay.io/strimzi/* to gsoci:   quay.io/strimzi/operator          → gsoci.azurecr.io/giantswarm/strimzi/operator   quay.io/strimzi/kafka             → gsoci.azurecr.io/giantswarm/strimzi/kafka   quay.io/strimzi/kafka-bridge      → gsoci.azurecr.io/giantswarm/strimzi/kafka-bridge   quay.io/strimzi/kaniko-executor   → gsoci.azurecr.io/giantswarm/strimzi/kaniko-executor   quay.io/strimzi/buildah           → gsoci.azurecr.io/giantswarm/strimzi/buildah   quay.io/strimzi/maven-builder     → gsoci.azurecr.io/giantswarm/strimzi/maven-builder |
| strimzi-kafka-operator.replicas | int | `2` | Number of operator replicas. Strimzi uses leader election so only one replica is active at a time; additional replicas are hot standbys. Requires replicas >= 2 for the PodDisruptionBudget to allow node drains. |
| strimzi-kafka-operator.resources | object | `{"limits":{"cpu":"1000m","memory":"384Mi"},"requests":{"cpu":"200m","memory":"384Mi"}}` | Operator resource limits (tune based on number of Kafka clusters watched). |
| strimzi-kafka-operator.podSecurityContext | object | `{"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Operator pod security context (hardened for GS standards). |
| strimzi-kafka-operator.securityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]},"readOnlyRootFilesystem":true,"runAsNonRoot":true,"seccompProfile":{"type":"RuntimeDefault"}}` | Operator container security context. |
| strimzi-kafka-operator.extraEnvs | list | `[{"name":"STRIMZI_POD_SECURITY_PROVIDER_CLASS","value":"io.strimzi.plugin.security.profiles.impl.RestrictedPodSecurityProvider"}]` | Use the restricted pod security provider so all Strimzi-generated pods (Kafka brokers, EntityOperator, etc.) get capabilities.drop=ALL, allowPrivilegeEscalation=false, runAsNonRoot=true, and seccompProfile=RuntimeDefault injected automatically. This satisfies GS Kyverno strict policies without needing a KyvernoPolicyException. |
| strimzi-kafka-operator.operatorNetworkPolicy | object | `{"egress":[{}],"enabled":true}` | NetworkPolicy for the operator pod, managed by the upstream chart. egress must be [{} ] (single empty rule) to allow all egress — the upstream template uses `with` so an empty map `{}` renders as blocked. |
| strimzi-kafka-operator.generateNetworkPolicy | bool | `true` | Keep Strimzi-generated NetworkPolicies for Kafka component pods. |
| strimzi-kafka-operator.leaderElection | object | `{"enable":true}` | Enable leader election. Must be true when replicas > 1 to avoid multiple active operators writing conflicting state. |
| strimzi-kafka-operator.watchAnyNamespace | bool | `true` | Watch all namespaces for Kafka CRs. Set to false and configure watchNamespaces if you want to restrict the operator to specific namespaces (e.g. in a shared cluster where teams manage their own operators). |
| strimzi-kafka-operator.podDisruptionBudget | object | `{"enabled":true,"minAvailable":1}` | PodDisruptionBudget for the operator Deployment. Ensures at least one replica is available during voluntary disruptions (node drain, rolling upgrades). Requires replicas >= 2 to have any effect. |
| strimzi-kafka-operator.affinity | object | `{"podAntiAffinity":{"preferredDuringSchedulingIgnoredDuringExecution":[{"podAffinityTerm":{"labelSelector":{"matchLabels":{"name":"strimzi-cluster-operator","strimzi.io/kind":"cluster-operator"}},"topologyKey":"kubernetes.io/hostname"},"weight":100}]}}` | Pod anti-affinity to spread operator replicas across nodes. preferredDuringScheduling so single-node clusters are not blocked. |
| strimzi-kafka-operator.topologySpreadConstraints | list | `[{"labelSelector":{"matchLabels":{"name":"strimzi-cluster-operator"}},"maxSkew":1,"topologyKey":"kubernetes.io/hostname","whenUnsatisfiable":"ScheduleAnyway"}]` | TopologySpreadConstraints spread operator pods across nodes. Uses ScheduleAnyway so scheduling is never blocked on single-node clusters. NOTE: requires upstream PR https://github.com/strimzi/strimzi-kafka-operator/pull/12560 to be merged before these values have any effect. |
| strimzi-kafka-operator.labels | object | `{"observability.giantswarm.io/tenant":"giantswarm"}` | Pod labels applied to the operator Deployment's pod template. The tenant label enables GS log ingestion for the operator pod. Docs: https://docs.giantswarm.io/overview/observability/data-management/data-ingestion/ |
| strimzi-kafka-operator.dashboards | object | `{"annotations":{"observability.giantswarm.io/folder":"Kafka","observability.giantswarm.io/organization":"Shared Org"},"enabled":false,"label":"app.giantswarm.io/kind","labelValue":"dashboard"}` | Subchart dashboard template disabled permanently. Two reasons we manage dashboards in the wrapper instead: 1. The upstream JSONs lack "uid" fields required by the GS admission webhook    (dashboardconfigmap.observability.giantswarm.io). 2. The subchart ships JMX-based dashboards; we use the strimzi-metrics-reporter    dashboards (+ strimzi-kafka-exporter) which require metricsConfig.type: strimziMetricsReporter. Dashboards are managed via the wrapper's dashboards: block and synced with `make sync-dashboards`. |

