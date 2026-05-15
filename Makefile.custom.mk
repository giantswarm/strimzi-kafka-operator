##@ Strimzi

CHART_DIR := helm/strimzi-kafka-operator
UPSTREAM_CHART_REPO := https://strimzi.io/charts
UPSTREAM_CHART_NAME := strimzi-kafka-operator
UPSTREAM_CHART_VERSION := 1.0.0

# We sync and patch the whole upstream chart instead of having a classic dependency.
# Reason: we are not happy with the way the upstream chart manages CRDs,
#                and we can't prevent it from installing them from values.
#                 So we move them from crds/ to templates/ so we can upgrade them along the app.
sync-chart:
	@echo "====> Syncing subchart from upstream $(UPSTREAM_CHART_REPO)/$(UPSTREAM_CHART_REPO) $(UPSTREAM_CHART_VERSION)"
	rm -rf $(CHART_DIR)/charts/strimzi-kafka-operator $(CHART_DIR)/templates/crds
	helm pull --repo $(UPSTREAM_CHART_REPO) $(UPSTREAM_CHART_NAME) --version $(UPSTREAM_CHART_VERSION) --destination $(CHART_DIR)/charts --untar
	mv $(CHART_DIR)/charts/strimzi-kafka-operator/crds $(CHART_DIR)/templates/crds
	@# Patch in the helm.sh/resource-policy: keep annotation so CRDs survive helm uninstall.
	@# The sed patterns do:
	@# - inserts after the 'annotations:' key at the metadata level.
	@# - add Helm conditionals to wrap all CRDs so they can be optionally installed (Values.crds.install).
	sed \
		-e '/^metadata:$$/a\  annotations:\n    "helm.sh/resource-policy": keep' \
		-e '1s/^/{{- if .Values.crds.install }}\n/' \
		-e '$$a {{- end }}' \
		-i $(CHART_DIR)/templates/crds/*.yaml
	@echo "Subchart synced to $(CHART_DIR). Review the diff and commit."

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
	@echo "====> Syncing Grafana dashboards for strimzi-kafka-operator $(UPSTREAM_CHART_VERSION)"
	rm -rf $(CHART_DIR)/files/grafana-dashboards/
	mkdir -p $(CHART_DIR)/files/grafana-dashboards/
	@# 5 strimzi-metrics-reporter dashboards
	@for dash in strimzi-kafka strimzi-kraft strimzi-kafka-connect strimzi-kafka-mirror-maker-2 strimzi-kafka-bridge; do \
	  echo "  Downloading $$dash.json"; \
	  curl -sSfL "https://raw.githubusercontent.com/strimzi/strimzi-kafka-operator/$(UPSTREAM_CHART_VERSION)/examples/metrics/strimzi-metrics-reporter/grafana-dashboards/$$dash.json" \
	    -o "$(CHART_DIR)/files/grafana-dashboards/$$dash.json"; \
	done
	@# kafka exporter dashboard (topic + consumer group lag metrics)
	@# 5 strimzi-metrics-reporter dashboards
	@for dash in strimzi-cruise-control strimzi-kafka-exporter strimzi-operators; do \
	  echo "  Downloading $$dash.json"; \
		curl -sSfL "https://raw.githubusercontent.com/strimzi/strimzi-kafka-operator/$(UPSTREAM_CHART_VERSION)/examples/metrics/grafana-dashboards/$$dash.json" \
	    -o "$(CHART_DIR)/files/grafana-dashboards/$$dash.json"; \
	done
	@# Inject "uid" into each dashboard JSON (filename stem used as UID, e.g.
	@# strimzi-kafka.json → "uid": "strimzi-kafka"). Required by the GS admission webhook.
	@for f in $(CHART_DIR)/files/grafana-dashboards/*.json; do \
	  uid=$$(basename "$$f" .json); \
	  jq --arg uid "$$uid" '. + {uid: $$uid}' "$$f" > "$$f.tmp" && mv "$$f.tmp" "$$f"; \
	done
	@# Prepend a `cluster_id` template variable (label "Cluster") and inject
	@# `cluster_id="$$cluster_id"` into every panel expression and variable
	@# query, so dashboards scope to a single GS cluster in shared Prometheus.
	@$(MAKE) patch-dashboards
	@echo "Dashboards synced to $(CHART_DIR)/files/grafana-dashboards/. Review the diff and commit."

.PHONY: patch-dashboards
patch-dashboards: ## Apply GS-specific tweaks to dashboard JSONs (cluster_id variable + filter, label fixups).
	@echo "====> Patching dashboards"
	python3 hack/patch-dashboards.py $(CHART_DIR)/files/grafana-dashboards/*.json

.PHONY: install-helm-unittest
install-helm-unittest:
	@if helm plugin list | awk '{print $$1}' | grep -qx unittest; then \
	  echo "====> helm-unittest plugin already installed"; \
	else \
	  echo "====> Installing helm-unittest plugin"; \
	  helm plugin install https://github.com/helm-unittest/helm-unittest --verify=false --version=1.0.3; \
	fi

.PHONY: test-chart
test-chart: install-helm-unittest ## Run helm-unittest test suites against the chart.
	helm unittest $(CHART_DIR)

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
