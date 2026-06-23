# Monitoring a Kafka cluster

Strimzi exposes Kafka metrics through four mechanisms (`strimziMetricsReporter`,
`jmxPrometheusExporter`, Kafka Exporter, raw JMX). This page describes how they relate, when
to use each, and how to configure them on a Giant Swarm cluster.

## TL;DR

A typical cluster needs two mechanisms, and they stack:

```yaml
spec:
  kafka:
    metricsConfig:
      type: strimziMetricsReporter    # broker/controller internal metrics (Prometheus, port 9404)
  kafkaExporter:                      # consumer-group lag + topic/partition offsets (port 9404)
    topicRegex: ".*"
    groupRegex: ".*"
```

This matches [`examples/kafka-single-node`](examples/kafka-single-node) and
[`examples/kafka-exporter`](examples/kafka-exporter), and the Giant Swarm Grafana dashboards.
The rest of this document explains the reasoning.

## The four mechanisms at a glance

| Mechanism | CR field | What it provides | Port | Format |
|---|---|---|---|---|
| **Strimzi Metrics Reporter** | `kafka.metricsConfig.type: strimziMetricsReporter` | Broker/controller internals (request rates, under-replicated partitions, JVM) | 9404 | Prometheus |
| **JMX Prometheus Exporter** | `kafka.metricsConfig.type: jmxPrometheusExporter` | Same broker/controller internals, via JMX→Prometheus conversion | 9404 | Prometheus |
| **Kafka Exporter** | `kafkaExporter:` | Consumer-group lag, per-topic/partition offsets, broker count | 9404 | Prometheus |
| **Raw JMX** | `kafka.jmxOptions:` | The live JMX interface for ad-hoc tooling (JConsole, `kafka-run-class`) | 9999 | JMX (not Prometheus) |

## Are these similar, additional, or exclusive?

The mechanisms answer **two distinct questions**, and map onto them:

**1. "How are the brokers themselves doing?"** — request rates, under-replicated partitions,
JVM/GC, controller state. `metricsConfig` serves this, through **two interchangeable
implementations**:

- `strimziMetricsReporter`
- `jmxPrometheusExporter`

`metricsConfig.type` is a single field, so these two are **mutually exclusive** — pick one.
They produce broadly the same `kafka_server_*` / `kafka_controller_*` / `kafka_network_*` /
`jvm_*` series on port 9404; they differ only in *how* (see pros/cons below).

**2. "Are the consumers keeping up?"** — consumer-group lag. The brokers do **not** report
this, so it requires a separate component: **Kafka Exporter**. It is **additional** — it
stacks on top of whichever `metricsConfig` implementation is chosen. It also runs alone, but
that leaves broker health uncovered.

**Raw JMX** (`jmxOptions`) sits on a different axis: it exposes the live JMX port for
*interactive* inspection, not Prometheus scraping. It is **additional** and optional. Most
clusters never need it, since the Prometheus path already surfaces the same MBeans.

So:

- `strimziMetricsReporter` **XOR** `jmxPrometheusExporter` (pick one)
- **+** Kafka Exporter (recommended, for lag)
- **+** raw JMX (rarely, for debugging)

## Pros and cons

### Strimzi Metrics Reporter (`strimziMetricsReporter`)

The native reporter, built into the Kafka image since Strimzi 0.44. The broker emits
Prometheus metrics directly.

- ➕ No ConfigMap and no mapping rules to maintain — set the type and nothing else.
- ➕ Lower overhead than the JMX exporter (no JMX scrape + regex conversion on every pull).
- ➕ The Giant Swarm dashboards are built against it (see [Dashboards](#dashboards)).
- ➖ Newer; metric *names* can differ slightly from long-standing JMX-exporter dashboards
  inherited from elsewhere.

**Use this by default.**

### JMX Prometheus Exporter (`jmxPrometheusExporter`)

The older approach: a Java agent reads JMX MBeans and converts them to Prometheus using a
ConfigMap of mapping rules.

- ➕ Mature; many community dashboards and mapping rules target its exact metric names.
- ➕ Full control over which MBeans are exposed and how they're named, via the rules.
- ➖ Requires a ConfigMap of mapping rules referenced from `metricsConfig` — more to write
  and keep in sync.
- ➖ Higher runtime overhead (JMX scrape + regex relabeling per scrape).

Choose it only when migrating dashboards that depend on its metric names, or when fine-grained
MBean control is required.

### Kafka Exporter (`kafkaExporter`)

A separate component ([danielqsj/kafka_exporter](https://github.com/danielqsj/kafka_exporter))
the operator deploys as its own `<cluster>-kafka-exporter` pod. It connects to Kafka as a
client and derives metrics from offsets.

- ➕ The only built-in source of **consumer-group lag** (`kafka_consumergroup_lag*`) — the
  single most useful saturation signal.
- ➕ Scraped by the same PodMonitor as the brokers (port 9404), so no extra scrape config.
- ➖ A broad `topicRegex` / `groupRegex` queries every topic and group on every scrape,
  raising exporter CPU, broker load, and Prometheus cardinality. Narrow it on large clusters.
- ➖ Lag is offset-derived and updated per scrape, so it is near-real-time, not instantaneous.

**Enable on any cluster with real consumers.**

### Raw JMX (`jmxOptions`)

Exposes the broker's remote JMX port (9999) for interactive tools.

- ➕ Allows attaching JConsole / VisualVM / `kafka-run-class kafka.tools.JmxTool` for ad-hoc,
  drill-down debugging.
- ➖ Not a Prometheus source — nothing scrapes it automatically; it is a manual tool.
- ➖ Extra exposed port to secure.

Enable it only for one-off investigations.

## How to choose

```
Want broker/controller health metrics in Prometheus/Grafana?
├─ Yes (almost always)
│   ├─ Migrating dashboards tied to JMX-exporter metric names? → jmxPrometheusExporter
│   └─ Otherwise (default)                                      → strimziMetricsReporter
└─ No → omit metricsConfig

Running consumers whose progress matters?
└─ Yes (almost always) → add kafkaExporter

Need to attach JConsole/VisualVM for a live debugging session?
└─ Yes (rare) → add jmxOptions
```

On Giant Swarm clusters, use **`strimziMetricsReporter` + `kafkaExporter`** as the baseline —
the shipped dashboards expect it.

## How to configure

### Strimzi Metrics Reporter

```yaml
spec:
  kafka:
    metricsConfig:
      type: strimziMetricsReporter
```

No ConfigMap required — the reporter is built into the Kafka image. Metrics are served on the
`tcp-prometheus` port (9404).

### JMX Prometheus Exporter

```yaml
spec:
  kafka:
    metricsConfig:
      type: jmxPrometheusExporter
      valueFrom:
        configMapKeyRef:
          name: kafka-metrics
          key: kafka-metrics-config.yml   # your mapping rules
```

Supply the referenced ConfigMap of mapping rules. See the upstream
[`examples/metrics`](https://github.com/strimzi/strimzi-kafka-operator/tree/1.0.0/examples/metrics)
for a complete ruleset.

### Kafka Exporter

```yaml
spec:
  # ...
  kafkaExporter:
    topicRegex: ".*"   # topics to report on
    groupRegex: ".*"   # consumer groups to report on
```

On large clusters, narrow the regexes (or use `topicExcludeRegex` / `groupExcludeRegex`) to
keep cardinality and load down.

The metrics it produces — all distinct from the broker JMX series, since they come from the
exporter pod rather than the brokers:

Consumer-group metrics (the reason to run it):

- `kafka_consumergroup_lag` / `kafka_consumergroup_lag_sum` — per-partition and per-group lag
- `kafka_consumergroup_current_offset` / `kafka_consumergroup_current_offset_sum` — committed offset per group/partition
- `kafka_consumergroup_members` — number of members in a consumer group

Topic / partition metrics:

- `kafka_topic_partitions` — partition count per topic
- `kafka_topic_partition_current_offset` / `kafka_topic_partition_oldest_offset` — newest and oldest offsets
- `kafka_topic_partition_in_sync_replica` / `kafka_topic_partition_replicas` — in-sync and total replicas
- `kafka_topic_partition_leader` / `kafka_topic_partition_leader_is_preferred` — leader broker and preferred-leader flag
- `kafka_topic_partition_under_replicated_partition` — under-replication flag

Cluster metric:

- `kafka_brokers` — number of brokers in the cluster

### Raw JMX

```yaml
spec:
  kafka:
    jmxOptions: {}   # exposes the JMX port (9999); add authentication for production
```

## How scraping works on Giant Swarm

The chart ships a `PodMonitor`
([`templates/podmonitor.yaml`](helm/strimzi-kafka-operator/templates/podmonitor.yaml)) that
scrapes the `tcp-prometheus` port (9404) of every pod labelled `strimzi.io/kind: Kafka` (also
`KafkaConnect` / `KafkaMirrorMaker2`). Both the Strimzi Metrics Reporter (broker pods) and
Kafka Exporter (its own pod) serve on 9404, so **both are picked up automatically** once
enabled — no extra scrape configuration. This is gated by `podMonitor.workloads.enabled` in
[values.yaml](helm/strimzi-kafka-operator/values.yaml).

## Dashboards

The wrapper chart ships Grafana dashboards (under `files/grafana-dashboards/`, managed via the
`dashboards:` block in [values.yaml](helm/strimzi-kafka-operator/values.yaml)) built for the
**Strimzi Metrics Reporter** plus **Kafka Exporter**. The upstream chart's JMX-exporter
dashboards are disabled for this reason. With `jmxPrometheusExporter` instead, those
dashboards may not line up with the metric names produced.

## Verifying it works

```shell
kubectl apply --filename examples/kafka-exporter
kubectl wait kafka/my-cluster --for=condition=Ready --timeout=10m

# exporter pod is up
kubectl get pods -l strimzi.io/component-type=kafka-exporter

# broker metrics are being served
kubectl exec my-cluster-broker-0 -- curl -s localhost:9404/metrics | head
```

## Upstream references

- [Strimzi metrics & monitoring guide](https://strimzi.io/docs/operators/latest/deploying#assembly-metrics-str)
- [Strimzi Metrics Reporter](https://github.com/strimzi/metrics-reporter)
- [JMX Prometheus Exporter examples](https://github.com/strimzi/strimzi-kafka-operator/tree/1.0.0/examples/metrics)
- [Kafka Exporter (danielqsj)](https://github.com/danielqsj/kafka_exporter)
