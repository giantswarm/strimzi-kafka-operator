# strimzi-kafka-operator

![Version: 1.0.0](https://img.shields.io/badge/Version-1.0.0-informational?style=flat-square) ![AppVersion: 1.0.0](https://img.shields.io/badge/AppVersion-1.0.0-informational?style=flat-square)

Strimzi: Apache Kafka running on Kubernetes

**Homepage:** <https://strimzi.io/>

## Maintainers

| Name | Email | Url |
| ---- | ------ | --- |
| Strimzi Project Maintainers | <cncf-strimzi-maintainers@lists.cncf.io> | <https://github.com/strimzi/governance> |

## Source Code

* <https://github.com/strimzi/strimzi-kafka-operator>

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| replicas | int | `1` |  |
| watchNamespaces | list | `[]` |  |
| watchAnyNamespace | bool | `false` |  |
| defaultImageRegistry | string | `"quay.io"` |  |
| defaultImageRepository | string | `"strimzi"` |  |
| defaultImageTag | string | `"1.0.0"` |  |
| image.registry | string | `""` |  |
| image.repository | string | `""` |  |
| image.name | string | `"operator"` |  |
| image.tag | string | `""` |  |
| logVolume | string | `"co-config-volume"` |  |
| logConfigMap | string | `"strimzi-cluster-operator"` |  |
| logConfiguration | string | `""` |  |
| logLevel | string | `"${env:STRIMZI_LOG_LEVEL:-INFO}"` |  |
| fullReconciliationIntervalMs | int | `120000` |  |
| operationTimeoutMs | int | `300000` |  |
| kubernetesServiceDnsDomain | string | `"cluster.local"` |  |
| featureGates | string | `""` |  |
| tmpDirSizeLimit | string | `"1Mi"` |  |
| extraEnvs | list | `[]` |  |
| tolerations | list | `[]` |  |
| topologySpreadConstraints | list | `[]` |  |
| affinity | object | `{}` |  |
| annotations | object | `{}` |  |
| labels | object | `{}` |  |
| nodeSelector | object | `{}` |  |
| deploymentAnnotations | object | `{}` |  |
| deploymentLabels | object | `{}` |  |
| deploymentStrategy | object | `{}` |  |
| priorityClassName | string | `""` |  |
| hostUsers | string | `nil` |  |
| podSecurityContext | object | `{}` |  |
| securityContext | object | `{}` |  |
| rbac.create | string | `"yes"` |  |
| serviceAccountCreate | string | `"yes"` |  |
| serviceAccount | string | `"strimzi-cluster-operator"` |  |
| leaderElection.enable | bool | `true` |  |
| podDisruptionBudget.enabled | bool | `false` |  |
| podDisruptionBudget.minAvailable | int | `1` |  |
| podDisruptionBudget.maxUnavailable | string | `nil` |  |
| podDisruptionBudget.unhealthyPodEvictionPolicy | string | `"IfHealthyBudget"` |  |
| operatorNetworkPolicy.enabled | bool | `false` |  |
| operatorNetworkPolicy.ingress[0].ports[0].protocol | string | `"TCP"` |  |
| operatorNetworkPolicy.ingress[0].ports[0].port | string | `"http"` |  |
| operatorNetworkPolicy.egress | object | `{}` |  |
| dashboards.enabled | bool | `false` |  |
| dashboards.namespace | string | `nil` |  |
| dashboards.label | string | `"grafana_dashboard"` |  |
| dashboards.labelValue | string | `"1"` |  |
| dashboards.annotations | object | `{}` |  |
| dashboards.extraLabels | object | `{}` |  |
| kafka.image.registry | string | `""` |  |
| kafka.image.repository | string | `""` |  |
| kafka.image.name | string | `"kafka"` |  |
| kafka.image.tagPrefix | string | `""` |  |
| kafkaConnect.image.registry | string | `""` |  |
| kafkaConnect.image.repository | string | `""` |  |
| kafkaConnect.image.name | string | `"kafka"` |  |
| kafkaConnect.image.tagPrefix | string | `""` |  |
| topicOperator.image.registry | string | `""` |  |
| topicOperator.image.repository | string | `""` |  |
| topicOperator.image.name | string | `"operator"` |  |
| topicOperator.image.tag | string | `""` |  |
| userOperator.image.registry | string | `nil` |  |
| userOperator.image.repository | string | `nil` |  |
| userOperator.image.name | string | `"operator"` |  |
| userOperator.image.tag | string | `""` |  |
| kafkaInit.image.registry | string | `""` |  |
| kafkaInit.image.repository | string | `""` |  |
| kafkaInit.image.name | string | `"operator"` |  |
| kafkaInit.image.tag | string | `""` |  |
| kafkaBridge.image.registry | string | `""` |  |
| kafkaBridge.image.repository | string | `nil` |  |
| kafkaBridge.image.name | string | `"kafka-bridge"` |  |
| kafkaBridge.image.tag | string | `"1.0.0"` |  |
| kafkaExporter.image.registry | string | `""` |  |
| kafkaExporter.image.repository | string | `""` |  |
| kafkaExporter.image.name | string | `"kafka"` |  |
| kafkaExporter.image.tagPrefix | string | `""` |  |
| kafkaMirrorMaker2.image.registry | string | `""` |  |
| kafkaMirrorMaker2.image.repository | string | `""` |  |
| kafkaMirrorMaker2.image.name | string | `"kafka"` |  |
| kafkaMirrorMaker2.image.tagPrefix | string | `""` |  |
| cruiseControl.image.registry | string | `""` |  |
| cruiseControl.image.repository | string | `""` |  |
| cruiseControl.image.name | string | `"kafka"` |  |
| cruiseControl.image.tagPrefix | string | `""` |  |
| kanikoExecutor.image.registry | string | `""` |  |
| kanikoExecutor.image.repository | string | `""` |  |
| kanikoExecutor.image.name | string | `"kaniko-executor"` |  |
| kanikoExecutor.image.tag | string | `""` |  |
| buildah.image.registry | string | `""` |  |
| buildah.image.repository | string | `""` |  |
| buildah.image.name | string | `"buildah"` |  |
| buildah.image.tag | string | `""` |  |
| mavenBuilder.image.registry | string | `""` |  |
| mavenBuilder.image.repository | string | `""` |  |
| mavenBuilder.image.name | string | `"maven-builder"` |  |
| mavenBuilder.image.tag | string | `""` |  |
| resources.limits.memory | string | `"384Mi"` |  |
| resources.limits.cpu | string | `"1000m"` |  |
| resources.requests.memory | string | `"384Mi"` |  |
| resources.requests.cpu | string | `"200m"` |  |
| livenessProbe.initialDelaySeconds | int | `10` |  |
| livenessProbe.periodSeconds | int | `30` |  |
| readinessProbe.initialDelaySeconds | int | `10` |  |
| readinessProbe.periodSeconds | int | `30` |  |
| createGlobalResources | bool | `true` |  |
| createAggregateRoles | bool | `false` |  |
| labelsExclusionPattern | string | `""` |  |
| generateNetworkPolicy | bool | `true` |  |
| connectBuildTimeoutMs | int | `300000` |  |
| generatePodDisruptionBudget | bool | `true` |  |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs v1.14.2](https://github.com/norwoodj/helm-docs/releases/v1.14.2)
