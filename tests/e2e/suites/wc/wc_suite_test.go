package wc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/giantswarm/apptest-framework/v4/pkg/state"
	"github.com/giantswarm/apptest-framework/v4/pkg/suite"
	crclient "github.com/giantswarm/clustertest/v4/pkg/client"
	clusternetwork "github.com/giantswarm/clustertest/v4/pkg/net"
	"github.com/giantswarm/clustertest/v4/pkg/wait"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// installNamespace is where the strimzi-kafka-operator chart is installed.
	installNamespace = "strimzi-system"

	// kafkaNamespace is the test namespace created to hold the Kafka CR.
	kafkaNamespace = "kafka-test"

	// kafkaClusterName is used for both the Kafka CR and KafkaNodePool CR.
	kafkaClusterName = "test-cluster"

	// kafkaPoolName is the KafkaNodePool name; the broker pod will be <cluster>-<pool>-0.
	kafkaPoolName = "dual-role"

	// operatorDeploymentName matches the upstream strimzi chart default.
	operatorDeploymentName = "strimzi-cluster-operator"

	// mimirURL is the Prometheus-compatible query API for the MC-hosted Mimir instance.
	// The test runner executes inside the MC so the in-cluster service is reachable directly.
	mimirURL = "http://mimir-gateway.mimir.svc/prometheus/api/v1/query"

	// mimirTenant routes the query to the GS observability tenant, matching the
	// observability.giantswarm.io/tenant label on the PodMonitor resources.
	mimirTenant = "giantswarm"
)

var isUpgrade = false

func TestWC(t *testing.T) {
	suite.New().
		WithHelmRelease(true).
		WithHelmTargetNamespace(installNamespace).
		WithIsUpgrade(isUpgrade).
		WithValuesFile("./values.yaml").
		AfterClusterReady(func() {
			It("should have WC API connectivity", func() {
				_, err := state.GetFramework().WC(state.GetCluster().Name)
				Expect(err).NotTo(HaveOccurred())
			})
		}).
		Tests(func() {
			Describe("strimzi-kafka-operator", Ordered, func() {
				var wcClient *crclient.Client

				BeforeAll(func() {
					var err error
					wcClient, err = state.GetFramework().WC(state.GetCluster().Name)
					Expect(err).NotTo(HaveOccurred())
				})

				// ------------------------------------------------------------
				// 1. Operator readiness
				// ------------------------------------------------------------
				It("operator Deployment should become ready", func() {
					ctx := state.GetContext()
					Eventually(func() (bool, error) {
						var dep appsv1.Deployment
						if err := wcClient.Get(ctx, types.NamespacedName{
							Namespace: installNamespace,
							Name:      operatorDeploymentName,
						}, &dep); err != nil {
							return false, nil //nolint:nilerr // not ready yet, keep polling
						}
						if dep.Spec.Replicas == nil {
							return false, nil
						}
						return dep.Status.ReadyReplicas > 0 &&
							dep.Status.ReadyReplicas == *dep.Spec.Replicas, nil
					}).WithPolling(10*time.Second).WithTimeout(5*time.Minute).Should(BeTrue(),
						"operator Deployment %s/%s never became ready", installNamespace, operatorDeploymentName)
				})

				// ------------------------------------------------------------
				// 2. Kafka integration
				// ------------------------------------------------------------
				Describe("kafka integration", Ordered, func() {
					// Stubs used by DoesResourceExist / IsResourceDeleted.
					var (
						kafkaNodePoolRef *unstructured.Unstructured
						kafkaRef         *unstructured.Unstructured
						namespaceRef     *corev1.Namespace
					)

					BeforeAll(func() {
						ctx := state.GetContext()

						By("creating test namespace " + kafkaNamespace)
						namespaceRef = &corev1.Namespace{
							ObjectMeta: metav1.ObjectMeta{Name: kafkaNamespace},
						}
						err := wcClient.Create(ctx, namespaceRef)
						if err != nil && !apierrors.IsAlreadyExists(err) {
							Expect(err).NotTo(HaveOccurred())
						}

						By("deploying KafkaNodePool CR")
						kafkaNodePoolRef = kafkaNodePoolManifest()
						err = wcClient.Create(ctx, kafkaNodePoolRef)
						if err != nil && !apierrors.IsAlreadyExists(err) {
							Expect(err).NotTo(HaveOccurred())
						}

						By("deploying Kafka CR")
						kafkaRef = kafkaManifest()
						err = wcClient.Create(ctx, kafkaRef)
						if err != nil && !apierrors.IsAlreadyExists(err) {
							Expect(err).NotTo(HaveOccurred())
						}

						By("waiting for Kafka CRs to be accepted by the operator")
						Eventually(wait.DoesResourceExist(ctx, wcClient, kafkaNodePoolRef)).
							WithPolling(5 * time.Second).WithTimeout(2 * time.Minute).Should(BeTrue())
						Eventually(wait.DoesResourceExist(ctx, wcClient, kafkaRef)).
							WithPolling(5 * time.Second).WithTimeout(2 * time.Minute).Should(BeTrue())
					})

					AfterAll(func() {
						ctx := state.GetContext()
						By("deleting test namespace " + kafkaNamespace)
						_ = wcClient.Delete(ctx, namespaceRef)
						Eventually(wait.IsResourceDeleted(ctx, wcClient, namespaceRef)).
							WithPolling(10 * time.Second).WithTimeout(5 * time.Minute).Should(BeTrue())
					})

					It("Kafka broker pod should become ready", func() {
						ctx := state.GetContext()
						podName := fmt.Sprintf("%s-%s-0", kafkaClusterName, kafkaPoolName)
						Eventually(func() (bool, error) {
							var pod corev1.Pod
							if err := wcClient.Get(ctx, types.NamespacedName{
								Namespace: kafkaNamespace,
								Name:      podName,
							}, &pod); err != nil {
								return false, nil //nolint:nilerr
							}
							for _, c := range pod.Status.Conditions {
								if c.Type == corev1.PodReady {
									return c.Status == corev1.ConditionTrue, nil
								}
							}
							return false, nil
						}).WithPolling(15*time.Second).WithTimeout(10*time.Minute).Should(BeTrue(),
							"Kafka broker pod %s/%s never became ready", kafkaNamespace, podName)
					})

					It("entity-operator Deployment should become ready", func() {
						ctx := state.GetContext()
						depName := fmt.Sprintf("%s-entity-operator", kafkaClusterName)
						Eventually(func() (bool, error) {
							var dep appsv1.Deployment
							if err := wcClient.Get(ctx, types.NamespacedName{
								Namespace: kafkaNamespace,
								Name:      depName,
							}, &dep); err != nil {
								return false, nil //nolint:nilerr
							}
							if dep.Spec.Replicas == nil {
								return false, nil
							}
							return dep.Status.ReadyReplicas > 0 &&
								dep.Status.ReadyReplicas == *dep.Spec.Replicas, nil
						}).WithPolling(10*time.Second).WithTimeout(5*time.Minute).Should(BeTrue(),
							"entity-operator Deployment %s-entity-operator never became ready", kafkaClusterName)
					})

					It("Kafka broker metrics should appear in Mimir", func() {
						// The PodMonitor scrapes every 60 s; allow several scrape cycles for
						// metrics to propagate through Alloy → remote-write → Mimir.
						promQL := fmt.Sprintf(`kafka_server_replicamanager_leadercount{namespace=%q}`, kafkaNamespace)
						Eventually(queryMimir(state.GetContext(), promQL)).
							WithPolling(30*time.Second).WithTimeout(5*time.Minute).Should(BeTrue(),
							"PromQL %q returned no results in Mimir after 5 minutes", promQL)
					})
				})
			})
		}).
		Run(t, "strimzi-kafka-operator WC test")
}

// kafkaNodePoolManifest returns the KafkaNodePool CR for a single-node KRaft cluster.
// Uses apiVersion v1 (served; matches examples in this repo).
func kafkaNodePoolManifest() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Version: "v1",
		Kind:    "KafkaNodePool",
	})
	u.SetName(kafkaPoolName)
	u.SetNamespace(kafkaNamespace)
	u.SetLabels(map[string]string{
		"strimzi.io/cluster": kafkaClusterName,
	})
	_ = unstructured.SetNestedField(u.Object, map[string]interface{}{
		"replicas": int64(1),
		"roles":    []interface{}{"controller", "broker"},
		"storage": map[string]interface{}{
			"type": "jbod",
			"volumes": []interface{}{
				map[string]interface{}{
					"id":            int64(0),
					"type":          "ephemeral",
					"kraftMetadata": "shared",
				},
			},
		},
	}, "spec")
	return u
}

// kafkaManifest returns the Kafka CR for a single-node KRaft cluster with built-in
// metrics reporter (strimziMetricsReporter) on port 9404.
func kafkaManifest() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Version: "v1",
		Kind:    "Kafka",
	})
	u.SetName(kafkaClusterName)
	u.SetNamespace(kafkaNamespace)
	u.SetAnnotations(map[string]string{
		"strimzi.io/node-pools": "enabled",
		"strimzi.io/kraft":      "enabled",
	})
	_ = unstructured.SetNestedField(u.Object, map[string]interface{}{
		"kafka": map[string]interface{}{
			"version":         "4.2.0",
			"metadataVersion": "4.2-IV1",
			"listeners": []interface{}{
				map[string]interface{}{
					"name": "plain",
					"port": int64(9092),
					"type": "internal",
					"tls":  false,
				},
			},
			"config": map[string]interface{}{
				"auto.create.topics.enable":                "false",
				"default.replication.factor":               int64(1),
				"min.insync.replicas":                      int64(1),
				"offsets.topic.replication.factor":         int64(1),
				"transaction.state.log.replication.factor": int64(1),
				"transaction.state.log.min.isr":            int64(1),
			},
			"metricsConfig": map[string]interface{}{
				"type": "strimziMetricsReporter",
			},
		},
		"entityOperator": map[string]interface{}{
			"topicOperator": map[string]interface{}{},
			"userOperator":  map[string]interface{}{},
		},
	}, "spec")
	return u
}

// mimirResponse is the subset of the Prometheus HTTP API response we care about.
type mimirResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []json.RawMessage `json:"result"`
	} `json:"data"`
}

// queryMimir returns a WaitCondition that polls Mimir with the given PromQL expression
// and resolves to true when at least one result is returned.
func queryMimir(ctx context.Context, promQL string) func() (bool, error) {
	httpClient := clusternetwork.NewHTTPClient()
	return func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, mimirURL, nil)
		if err != nil {
			return false, fmt.Errorf("building Mimir request: %w", err)
		}
		q := req.URL.Query()
		q.Set("query", promQL)
		req.URL.RawQuery = q.Encode()
		req.Header.Set("X-Scope-OrgID", mimirTenant)

		resp, err := httpClient.Do(req)
		if err != nil {
			// Network error — not fatal, keep polling.
			return false, nil //nolint:nilerr
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return false, nil
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, nil //nolint:nilerr
		}

		var result mimirResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return false, nil //nolint:nilerr
		}

		return result.Status == "success" && len(result.Data.Result) > 0, nil
	}
}
