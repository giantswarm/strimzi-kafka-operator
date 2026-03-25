##@ Strimzi

CHART_DIR := helm/strimzi-kafka-operator

.PHONY: sync-crds
sync-crds: ## Re-extract CRDs from upstream chart tgz after a version bump.
	@echo "====> Syncing CRDs from upstream strimzi-kafka-operator chart"
	helm dep update $(CHART_DIR)
	rm -rf $(CHART_DIR)/templates/crds/
	mkdir -p $(CHART_DIR)/templates/crds/
	tar -xzf $(CHART_DIR)/charts/strimzi-kafka-operator-*.tgz \
	  --strip-components=2 \
	  -C $(CHART_DIR)/templates/crds/ \
	  strimzi-kafka-operator/crds/
	@# Patch in the helm.sh/resource-policy: keep annotation so CRDs survive helm uninstall.
	@# The sed pattern inserts after the 'annotations:' key at the metadata level.
	find $(CHART_DIR)/templates/crds/ -name '*.yaml' -exec \
	  sed -i '/^  annotations:$$/a\    "helm.sh/resource-policy": keep' {} \;
	@echo "CRDs synced to $(CHART_DIR)/templates/crds/. Review the diff and commit."

.PHONY: sync-dashboards
sync-dashboards: ## Download and patch Grafana dashboard JSONs from upstream GitHub after a version bump.
	@# We use the strimzi-metrics-reporter dashboards (not the JMX-based ones in the subchart)
	@# because the Kafka CR is configured with metricsConfig.type: strimziMetricsReporter.
	@# We also include strimzi-kafka-exporter (topic/consumer-group lag metrics).
	@#
	@# UIDs are injected (filename stem) because the GS admission webhook
	@# (dashboardconfigmap.observability.giantswarm.io) requires them.
	@# TODO: remove uid injection once Strimzi adds stable uid fields upstream.
	@# Track at: https://github.com/strimzi/strimzi-kafka-operator (open an issue)
	$(eval VERSION := $(shell grep appVersion $(CHART_DIR)/Chart.yaml | awk '{print $$2}' | tr -d '"'))
	@echo "====> Syncing Grafana dashboards for strimzi-kafka-operator $(VERSION)"
	rm -rf $(CHART_DIR)/files/grafana-dashboards/
	mkdir -p $(CHART_DIR)/files/grafana-dashboards/
	@# 5 strimzi-metrics-reporter dashboards
	@for dash in strimzi-kafka strimzi-kraft strimzi-kafka-connect strimzi-kafka-mirror-maker-2 strimzi-kafka-bridge; do \
	  echo "  Downloading $$dash.json"; \
	  curl -sSfL "https://raw.githubusercontent.com/strimzi/strimzi-kafka-operator/$(VERSION)/examples/metrics/strimzi-metrics-reporter/grafana-dashboards/$$dash.json" \
	    -o "$(CHART_DIR)/files/grafana-dashboards/$$dash.json"; \
	done
	@# kafka exporter dashboard (topic + consumer group lag metrics)
	@echo "  Downloading strimzi-kafka-exporter.json"
	curl -sSfL "https://raw.githubusercontent.com/strimzi/strimzi-kafka-operator/$(VERSION)/examples/metrics/grafana-dashboards/strimzi-kafka-exporter.json" \
	  -o "$(CHART_DIR)/files/grafana-dashboards/strimzi-kafka-exporter.json"
	@# Inject "uid" into each dashboard JSON (filename stem used as UID, e.g.
	@# strimzi-kafka.json → "uid": "strimzi-kafka"). Required by the GS admission webhook.
	@for f in $(CHART_DIR)/files/grafana-dashboards/*.json; do \
	  uid=$$(basename "$$f" .json); \
	  jq --arg uid "$$uid" '. + {uid: $$uid}' "$$f" > "$$f.tmp" && mv "$$f.tmp" "$$f"; \
	done
	@echo "Dashboards synced to $(CHART_DIR)/files/grafana-dashboards/. Review the diff and commit."

.PHONY: show-images
show-images: ## List all container images used by this chart (requires helm dep update first).
	@echo "====> Images referenced by strimzi-kafka-operator v$$(grep appVersion $(CHART_DIR)/Chart.yaml | awk '{print $$2}')"
	@echo ""
	@echo "Images to retag from quay.io/strimzi → gsoci.azurecr.io/giantswarm/strimzi:"
	@echo "  quay.io/strimzi/operator:<appVersion>"
	@echo "  quay.io/strimzi/kafka:<appVersion>-kafka-<kafkaVersion>   (multiple kafka versions)"
	@echo "  quay.io/strimzi/kafka-bridge:0.33.1"
	@echo "  quay.io/strimzi/kaniko-executor:<appVersion>"
	@echo "  quay.io/strimzi/buildah:<appVersion>"
	@echo "  quay.io/strimzi/maven-builder:<appVersion>"
