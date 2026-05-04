# Contributing

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

## After a version bump (Renovate PR)

When Renovate bumps the chart version in `Chart.yaml`, re-sync the upstream chart:

```bash
make sync-chart
git add helm/strimzi-kafka-operator/
git commit -m "Sync upstream chart for strimzi-kafka-operator v<new-version>"
```
