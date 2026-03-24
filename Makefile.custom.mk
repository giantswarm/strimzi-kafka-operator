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
