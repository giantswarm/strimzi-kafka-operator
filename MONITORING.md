# Monitoring a Kafka Cluster Deployed with Strimzi

This guide documents how to monitor a Kafka cluster managed by this operator. It
explains the available metrics collection methods, how they differ, and how to
configure them. Use it to select a method and understand what this chart already
provides out of the box.

> **Recommendation:** Use the **Strimzi Metrics Reporter** (`strimziMetricsReporter`).
> It is the default and recommended method. The Grafana dashboards provided on
> the Observability Platform are built for the Strimzi Metrics Reporter and
> require it. Use the
> JMX Prometheus Exporter only when running a Strimzi version older than 0.47.0,
> or when integrating with external dashboards that depend on the JMX exporter
> metric names.

---

## What is already provided

Scraping and visualization are already handled for you. You do not need to
configure Prometheus scraping or import dashboards manually:

- **Scraping is pre-configured and ready to enable** by this chart through `PodMonitor`
  resources (see [Metrics scraping](#metrics-scraping)).
- **Grafana dashboards are provided centrally** by the Observability Platform —
  they are maintained in the [`giantswarm/dashboards`](https://github.com/giantswarm/dashboards)
  repository and appear in Grafana (see
  [Grafana dashboards](#grafana-dashboards)).

What you must decide and configure yourself is the **metrics collection method**
on your Kafka resources (`metricsConfig`), and whether to enable the
**Kafka Exporter**.

---

## Overview

Strimzi exposes metrics through two layers:

1. **Component metrics** — JVM and Kafka MBean metrics emitted by each component
   (brokers, controllers, Connect, MirrorMaker 2, Bridge, Cruise Control). These
   are configured per component through the `metricsConfig` field on the
   corresponding Kafka component resource. Choose **one** of two mechanisms:
   - [Strimzi Metrics Reporter](#method-1--strimzi-metrics-reporter-recommended-default)
     (`strimziMetricsReporter`) — recommended.
   - [JMX Prometheus Exporter](#method-2--jmx-prometheus-exporter-legacy--compatibility)
     (`jmxPrometheusExporter`) — legacy/compatibility.

2. **[Supplementary metrics sources](#supplementary-metrics-sources)** — surface
   metrics not available through the component metrics layer:
   - **[Kafka Exporter](#kafka-exporter)** — consumer group lag, offsets, and
     topic-level metrics.
   - **[Operator metrics](#operator-metrics)** — emitted by the Cluster Operator
     and the Entity Operator (Topic and User Operators); enabled by default, no
     configuration required.

The `metricsConfig` mechanisms are **mutually exclusive** per component. The
supplementary sources are independent and may be combined with either mechanism.

---

## Method 1 — Strimzi Metrics Reporter (recommended, default)

### What it does

The Strimzi Metrics Reporter is a Kafka `MetricsReporter` plugin that runs
**inside** the Kafka process. It reads Kafka metrics directly through the
internal metrics API and exposes them in Prometheus format over an HTTP endpoint
on the `tcp-prometheus` port (`9404`) at `/metrics`. No JMX agent is involved.

### How it differs

- **No JVM agent.** Metrics are read natively rather than scraped from JMX
  MBeans, which lowers CPU and memory overhead.
- **Stable metric names.** Metric names are fixed and predictable, removing the
  need for the large relabeling rule set required by the JMX exporter.
- **Inline configuration.** Filtering is done with an `allowList` of regular
  expressions defined directly on the resource, rather than an external
  ConfigMap of translation rules.
- **Dashboards.** The Grafana dashboards provided on the Observability Platform
  are built for and require this method.
- **Availability.** Introduced in Strimzi **0.47.0**. Not available on older
  versions.

### How to configure

Set `metricsConfig.type` to `strimziMetricsReporter` on the Kafka resource:

```yaml
apiVersion: kafka.strimzi.io/v1
kind: Kafka
metadata:
  name: my-cluster
spec:
  kafka:
    metricsConfig:
      type: strimziMetricsReporter
```

Optionally narrow the exposed metrics with an `allowList`:

```yaml
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
(`KafkaConnect`, `KafkaMirrorMaker2`, `KafkaBridge`).

---

## Method 2 — JMX Prometheus Exporter (legacy / compatibility)

### What it does

The JMX Prometheus Exporter runs as a Java agent attached to the Kafka process.
It reads Kafka metrics exposed as **JMX MBeans**, applies a set of relabeling
rules to convert them into Prometheus metric names, and exposes the result on
the `tcp-prometheus` port (`9404`) at `/metrics`.

### How it differs

- **Java agent.** Adds JVM overhead and increases the metrics scrape surface.
- **Rule-driven naming.** Relies on an extensive set of regex-based translation
  rules to map JMX MBean names to Prometheus metric names, maintained in an
  external ConfigMap.
- **Dashboards.** The dashboards provided on the Observability Platform do
  **not** target this method. Using the JMX exporter requires supplying your own
  dashboards.
- **Availability.** Supported across all current and older Strimzi versions.

### How to configure

Create a ConfigMap containing the exporter rules, then reference it from the
`metricsConfig` field:

```yaml
apiVersion: kafka.strimzi.io/v1
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

See this [example](https://github.com/strimzi/strimzi-kafka-operator/blob/1.0.0/examples/metrics/kafka-metrics.yaml#L82) for exporter rules.

Apply the rules ConfigMap before deploying the Kafka resource.

---

## Choosing a method

| Criterion               | Strimzi Metrics Reporter        | JMX Prometheus Exporter          |
| ----------------------- | ------------------------------- | -------------------------------- |
| Status                  | Default, recommended            | Legacy / compatibility           |
| Mechanism               | In-process reporter plugin      | JMX Java agent                   |
| Overhead                | Lower                           | Higher                           |
| Metric names            | Fixed, predictable              | Derived from JMX via regex rules |
| Configuration           | Inline `allowList`              | External ConfigMap of rules      |
| Platform dashboards     | Supported (required)            | Not provided                     |
| Minimum Strimzi version | 0.47.0                          | All versions                     |

Select the **Strimzi Metrics Reporter** unless one of the following applies, in
which case select the **JMX Prometheus Exporter**:

- The cluster runs a Strimzi version older than 0.47.0.
- You integrate with external dashboards or tooling that depend on the JMX
  exporter metric names.

Do not enable both mechanisms on the same component.

---

## Supplementary metrics sources

### Kafka Exporter

The component metrics layer reports broker-internal metrics (request rates,
under-replicated partitions, JVM) but **cannot** report consumer group lag,
because lag is derived from the difference between committed consumer offsets and
the log end offset. The Kafka Exporter fills this gap.

When enabled, the operator deploys it as a separate `<cluster>-kafka-exporter`
pod that connects to Kafka as a client and exposes:

- **Consumer group lag** (`kafka_consumergroup_lag`, `kafka_consumergroup_lag_sum`)
  — the primary reason to enable it.
- Per-consumer-group committed offsets.
- Per-topic and per-partition offsets.

Enable it under `spec.kafkaExporter` on the Kafka resource:

```yaml
spec:
  kafkaExporter:
    topicRegex: ".*"
    groupRegex: ".*"
```

The exporter pod is labelled `strimzi.io/kind: Kafka` and serves metrics on the
standard `tcp-prometheus` port (`9404`), so it is scraped by the same PodMonitor
that scrapes the brokers — no additional scrape configuration is required. Its
output is visualized by the `strimzi-kafka-exporter` dashboard.

> **Scaling note:** `topicRegex` / `groupRegex` of `".*"` report on every topic
> and consumer group. This is acceptable for small or demo clusters. On large
> clusters, narrow these (or use `topicExcludeRegex` / `groupExcludeRegex`): a
> broad regex queries every partition and group on every scrape, raising exporter
> CPU, broker load, and Prometheus cardinality. See https://strimzi.io/docs/operators/latest/configuring#type-KafkaExporterSpec-reference

### Operator metrics

The operators expose their own Prometheus metrics, enabled by default with no
configuration required:

- **Cluster Operator** — reconciliation counts, durations, and error rates for
  the custom resources it manages. Scraped from the `strimzi-cluster-operator`
  pod on the `http` port (`8080`) at `/metrics`.
- **Entity Operator** — metrics from the Topic Operator and User Operator (e.g.
  managed topic and user reconciliations). Scraped from the entity-operator pod
  on the `healthcheck` port (`8080`) at `/metrics`.

Use these to monitor the health of the operator itself: stalled reconciliations,
rising error rates, or reconciliation latency. They are visualized by the
`strimzi-operators` dashboard.

---

## Metrics scraping

Scraping is pre-configured and ready to enable through `PodMonitor` resources created by this
chart (`podMonitor.enabled: true`). The chart creates monitors for:

- The Cluster Operator pod (`http` / `8080`).
- Kafka, KafkaConnect, and KafkaMirrorMaker2 pods (`tcp-prometheus` / `9404`) —
  this also covers the Kafka Exporter pod.
- KafkaBridge pods (`rest-api` / `8080`).
- The Entity Operator pod (`healthcheck` / `8080`).

The workload PodMonitors select pods across **all namespaces**
(`podMonitor.workloads.enabled: true`), so Kafka clusters deployed in any
namespace are scraped automatically. Workload metrics still require
`metricsConfig` to be set on the respective custom resource — without it, the
pods expose no metrics endpoint to scrape.

Configure the monitors through the `podMonitor` block in `values.yaml`. The
`observability.giantswarm.io/tenant` label is mandatory on the Observability
Platform: it routes the scraped metrics to a tenant in Mimir and determines
which Grafana organization can query them. For background on metric ingestion
and the tenant model, see the Giant Swarm docs:

- [Data ingestion (ServiceMonitor / PodMonitor + tenant label)](https://docs.giantswarm.io/overview/observability/data-management/data-ingestion/)
- [Multi-tenancy (tenants and Grafana organizations)](https://docs.giantswarm.io/overview/observability/configuration/multi-tenancy/)
- [Observe your clusters and apps (end-to-end walkthrough)](https://docs.giantswarm.io/getting-started/observe-your-clusters-and-apps/)

---

## Grafana dashboards

The Kafka dashboards are maintained centrally and deployed to the Observability
Platform — you do not deploy or import them per cluster. They live in the
[`giantswarm/dashboards`](https://github.com/giantswarm/dashboards) repository,
which provisions them into the **Shared Org** Grafana organization in the
**Kafka** folder. Open Grafana on the Observability Platform and browse to that
folder to find them.

The dashboards are built for the **Strimzi Metrics Reporter** and depend on its
metric names. They will not populate correctly when using the JMX Prometheus
Exporter — this is the main reason the Strimzi Metrics Reporter is the
recommended method.

Available dashboards:

- Broker metrics
- KRaft controller metrics
- Kafka Exporter — consumer group lag, offsets, topic metrics
- Operator metrics — Cluster and Entity Operator
- Kafka Connect, MirrorMaker 2, and Kafka Bridge
- Cruise Control / rebalance metrics

To add or update a dashboard, contribute it to the `giantswarm/dashboards`
repository (the path under a team chart determines the target Grafana
organization and folder). See the Giant Swarm dashboard management docs:

- [Dashboard management](https://docs.giantswarm.io/overview/observability/dashboard-management/)
- [Dashboard creation](https://docs.giantswarm.io/overview/observability/dashboard-management/dashboard-creation/)

---

## References

### Giant Swarm Observability Platform

- Observability overview:
  <https://docs.giantswarm.io/overview/observability/>
- Observe your clusters and apps (end-to-end walkthrough):
  <https://docs.giantswarm.io/getting-started/observe-your-clusters-and-apps/>
- Data ingestion (ServiceMonitor / PodMonitor + tenant label):
  <https://docs.giantswarm.io/overview/observability/data-management/data-ingestion/>
- Multi-tenancy (tenants and Grafana organizations):
  <https://docs.giantswarm.io/overview/observability/configuration/multi-tenancy/>
- Dashboard management:
  <https://docs.giantswarm.io/overview/observability/dashboard-management/>
- Central dashboards repository:
  <https://github.com/giantswarm/dashboards>

### Strimzi

- Strimzi metrics documentation:
  <https://strimzi.io/docs/operators/latest/deploying#assembly-metrics-str>
- Strimzi configuration reference:
  <https://strimzi.io/docs/operators/latest/configuring.html>
- Strimzi Metrics Reporter announcement:
  <https://strimzi.io/blog/2025/10/06/strimzi-metrics-reporter/>
- Strimzi Metrics Reporter repository:
  <https://github.com/strimzi/metrics-reporter>
