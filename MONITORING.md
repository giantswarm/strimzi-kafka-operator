# Monitoring a Kafka Cluster Deployed with Strimzi

This guide documents the available methods for collecting metrics from a Kafka
cluster managed by the Strimzi operator, explains how they differ, and shows how
to configure each one. Use it to select a metrics collection method and wire the
cluster into Prometheus and Grafana.

> **Recommendation:** Use the **Strimzi Metrics Reporter** (`strimziMetricsReporter`).
> It is the default and recommended method for new deployments. Use the
> JMX Prometheus Exporter only when you require backward compatibility with
> existing dashboards and alert rules, or when running a Strimzi version older
> than 0.47.0.

---

## Overview

Strimzi exposes metrics through two layers:

1. **Component metrics** — JVM and Kafka MBean metrics emitted by each component
   (brokers, controllers, Connect, MirrorMaker 2, Bridge, Cruise Control). These
   are configured per component through the `metricsConfig` field on the
   corresponding custom resource. Choose **one** of two mechanisms:
   - Strimzi Metrics Reporter (`strimziMetricsReporter`) — recommended.
   - JMX Prometheus Exporter (`jmxPrometheusExporter`) — legacy/compatibility.

2. **Supplementary exporters** — additional components that surface metrics not
   available through the component metrics layer:
   - **Kafka Exporter** — consumer group lag, offsets, and topic-level metrics.
   - **Operator metrics** — emitted by the Cluster, Topic, and User Operators
     (enabled by default, no configuration required).
   - **kube-state-metrics** — Strimzi custom resource state at the Kubernetes
     API level.

The `metricsConfig` mechanisms are **mutually exclusive** per component. The
supplementary exporters are independent and may be combined with either
mechanism.

---

## Method 1 — Strimzi Metrics Reporter (recommended, default)

### What it does

The Strimzi Metrics Reporter is a Kafka `MetricsReporter` plugin that runs
**inside** the Kafka process. It reads Kafka metrics directly through the
internal metrics API and exposes them in Prometheus format over an HTTP
endpoint. No JMX agent is involved.

### How it differs

- **No JVM agent.** Metrics are read natively rather than scraped from JMX
  MBeans, which lowers CPU and memory overhead.
- **Stable metric names.** Metric names are fixed and predictable, removing the
  need for the large relabeling rule set required by the JMX exporter.
- **Inline configuration.** Filtering is done with an `allowList` of regular
  expressions defined directly on the resource, rather than an external
  ConfigMap of translation rules.
- **Availability.** Introduced in Strimzi **0.47.0**. Not available on older
  versions.

### How to configure

Set `metricsConfig.type` to `strimziMetricsReporter` and provide an `allowList`
of metric name patterns to expose:

```yaml
apiVersion: kafka.strimzi.io/v1beta2
kind: Kafka
metadata:
  name: my-cluster
spec:
  kafka:
    metricsConfig:
      type: strimziMetricsReporter
      values:
        allowList:
          - "kafka_log.*"
          - "kafka_network.*"
          - "kafka_server.*"
          - "kafka_controller.*"
```

Apply the same `metricsConfig` block to other components as needed
(`KafkaConnect`, `KafkaMirrorMaker2`, `KafkaBridge`). Metrics are exposed on the
metrics port (`9404`) at `/metrics`.

---

## Method 2 — JMX Prometheus Exporter (legacy / compatibility)

### What it does

The JMX Prometheus Exporter runs as a Java agent attached to the Kafka process.
It reads Kafka metrics exposed as **JMX MBeans**, applies a set of relabeling
rules to convert them into Prometheus metric names, and exposes the result over
HTTP on port `9404` at `/metrics`.

### How it differs

- **Java agent.** Adds JVM overhead and increases the metrics scrape surface.
- **Rule-driven naming.** Relies on an extensive set of regex-based translation
  rules to map JMX MBean names to Prometheus metric names. These rules are
  maintained in an external ConfigMap.
- **Established ecosystem.** The official Strimzi example Grafana dashboards and
  Prometheus alert rules were historically built around the metric names this
  exporter produces.
- **Availability.** Supported across all current and older Strimzi versions.

### How to configure

Create a ConfigMap containing the exporter rules, then reference it from the
`metricsConfig` field. Strimzi ships ready-to-use rule files in the
`examples/metrics` directory of the operator repository.

```yaml
apiVersion: kafka.strimzi.io/v1beta2
kind: Kafka
metadata:
  name: my-cluster
spec:
  kafka:
    metricsConfig:
      type: jmxPrometheusExporter
      valueFrom:
        configMapKeyRef:
          name: kafka-metrics
          key: kafka-metrics-config.yml
```

Apply the corresponding example ConfigMap
(`examples/metrics/kafka-metrics.yaml`) before deploying the Kafka resource.

---

## Choosing a method

| Criterion                  | Strimzi Metrics Reporter        | JMX Prometheus Exporter            |
| -------------------------- | ------------------------------- | ---------------------------------- |
| Status                     | Default, recommended            | Legacy / compatibility             |
| Mechanism                  | In-process reporter plugin      | JMX Java agent                     |
| Overhead                   | Lower                           | Higher                             |
| Metric names               | Fixed, predictable              | Derived from JMX via regex rules   |
| Configuration              | Inline `allowList`              | External ConfigMap of rules        |
| Minimum Strimzi version    | 0.47.0                          | All versions                       |
| Existing dashboards/alerts | Require updated metric mappings | Match official examples directly   |

Select the **Strimzi Metrics Reporter** unless one of the following applies, in
which case select the **JMX Prometheus Exporter**:

- The cluster runs a Strimzi version older than 0.47.0.
- Existing Grafana dashboards or Prometheus alert rules depend on the JMX
  exporter metric names and must not be changed.

Do not enable both mechanisms on the same component.

---

## Supplementary exporters

### Kafka Exporter

Enable the Kafka Exporter to collect consumer group lag, consumer offsets, and
topic-level metrics, which are not available through either component metrics
mechanism. Add the `kafkaExporter` section to the Kafka resource:

```yaml
spec:
  kafkaExporter:
    topicRegex: ".*"
    groupRegex: ".*"
```

### Operator metrics

The Cluster, Topic, and User Operators expose Prometheus metrics by default. No
configuration is required; scrape the operator pods directly. Visualize with the
`strimzi-operators` dashboard.

### kube-state-metrics

Deploy kube-state-metrics to monitor the state of Strimzi custom resources at
the Kubernetes API level (resource readiness, counts, conditions). This is
independent of the in-cluster Kafka metrics.

---

## Collecting and visualizing metrics

Regardless of the method selected, complete the monitoring pipeline:

1. **Scrape.** Configure Prometheus to scrape port `9404` of the relevant pods.
   Use a `PodMonitor` or `ServiceMonitor` when running the Prometheus Operator.
2. **Alert.** Apply the example Prometheus alerting rules
   (`examples/metrics/prometheus-install/prometheus-rules.yaml`) and route
   notifications through Alertmanager.
3. **Visualize.** Add Prometheus as a Grafana data source and import the example
   dashboards from `examples/metrics/grafana-dashboards/`:
   - `strimzi-kafka.json` — broker metrics
   - `strimzi-kraft.json` — KRaft controller metrics
   - `strimzi-kafka-exporter.json` — consumer lag, offsets, topic metrics
   - `strimzi-operators.json` — operator metrics
   - `strimzi-cruise-control.json` — Cruise Control / rebalance metrics
   - Component dashboards for Connect, MirrorMaker 2, and Bridge

> **Note:** The official example dashboards and alert rules are built around the
> JMX Prometheus Exporter metric names. When using the Strimzi Metrics Reporter,
> verify and adjust dashboard queries and alert expressions to match the
> reporter's metric names.

---

## References

- Strimzi metrics documentation:
  <https://strimzi.io/docs/operators/latest/deploying#assembly-metrics-str>
- Strimzi configuration reference:
  <https://strimzi.io/docs/operators/latest/configuring.html>
- Example metrics configuration and dashboards:
  <https://github.com/strimzi/strimzi-kafka-operator/tree/main/examples/metrics>
- Strimzi Metrics Reporter announcement:
  <https://strimzi.io/blog/2025/10/06/strimzi-metrics-reporter/>
- Strimzi Metrics Reporter repository:
  <https://github.com/strimzi/metrics-reporter>
- Prometheus Metrics Reporter proposal:
  <https://github.com/strimzi/proposals/blob/main/064-prometheus-metrics-reporter.md>
