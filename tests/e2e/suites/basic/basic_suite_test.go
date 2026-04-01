package basic_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/giantswarm/apptest-framework/v4/pkg/state"
	"github.com/giantswarm/apptest-framework/v4/pkg/suite"
	crclient "github.com/giantswarm/clustertest/v4/pkg/client"
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
	installNamespace       = "strimzi-system"
	kafkaNamespace         = "kafka-test"
	kafkaClusterName       = "test-cluster"
	kafkaPoolName          = "dual-role"
	operatorDeploymentName = "strimzi-cluster-operator"
	mimirEndpoint          = "http://mimir-gateway.mimir.svc/prometheus/api/v1/query"
)

func TestBasic(t *testing.T) {
	suite.New().
		WithHelmRelease(true).
		WithHelmTargetNamespace(installNamespace).
		WithIsUpgrade(false).
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

				It("operator Deployment should become ready", func() {
					ctx := state.GetContext()
					Eventually(func() (bool, error) {
						var dep appsv1.Deployment
						if err := wcClient.Get(ctx, types.NamespacedName{
							Namespace: installNamespace,
							Name:      operatorDeploymentName,
						}, &dep); err != nil {
							return false, nil //nolint:nilerr
						}
						if dep.Spec.Replicas == nil {
							return false, nil
						}
						return dep.Status.ReadyReplicas > 0 &&
							dep.Status.ReadyReplicas == *dep.Spec.Replicas, nil
					}).WithPolling(10*time.Second).WithTimeout(5*time.Minute).Should(BeTrue(),
						"operator Deployment %s/%s never became ready", installNamespace, operatorDeploymentName)
				})

				Describe("kafka integration", Ordered, func() {
					var (
						kafkaNodePoolRef *unstructured.Unstructured
						kafkaRef         *unstructured.Unstructured
						namespaceRef     *corev1.Namespace
					)

					BeforeAll(func() {
						ctx := state.GetContext()

						namespaceRef = &corev1.Namespace{
							ObjectMeta: metav1.ObjectMeta{Name: kafkaNamespace},
						}
						err := wcClient.Create(ctx, namespaceRef)
						if err != nil && !apierrors.IsAlreadyExists(err) {
							Expect(err).NotTo(HaveOccurred())
						}

						kafkaNodePoolRef = kafkaNodePoolManifest()
						err = wcClient.Create(ctx, kafkaNodePoolRef)
						if err != nil && !apierrors.IsAlreadyExists(err) {
							Expect(err).NotTo(HaveOccurred())
						}

						kafkaRef = kafkaManifest()
						err = wcClient.Create(ctx, kafkaRef)
						if err != nil && !apierrors.IsAlreadyExists(err) {
							Expect(err).NotTo(HaveOccurred())
						}

						Eventually(wait.DoesResourceExist(ctx, wcClient, kafkaNodePoolRef)).
							WithPolling(5 * time.Second).WithTimeout(2 * time.Minute).Should(BeTrue())
						Eventually(wait.DoesResourceExist(ctx, wcClient, kafkaRef)).
							WithPolling(5 * time.Second).WithTimeout(2 * time.Minute).Should(BeTrue())
					})

					AfterAll(func() {
						ctx := state.GetContext()
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
						ctx := state.GetContext()
						mcClient := state.GetFramework().MC()

						podName := fmt.Sprintf("%s-metrics-test", state.GetCluster().Name)
						t := true
						f := false
						uid := int64(35)
						pod := &corev1.Pod{
							ObjectMeta: metav1.ObjectMeta{
								Name:      podName,
								Namespace: "default",
							},
							Spec: corev1.PodSpec{
								SecurityContext: &corev1.PodSecurityContext{
									RunAsUser:    &uid,
									RunAsGroup:   &uid,
									RunAsNonRoot: &t,
									SeccompProfile: &corev1.SeccompProfile{
										Type: corev1.SeccompProfileTypeRuntimeDefault,
									},
								},
								Containers: []corev1.Container{
									{
										Name:  "test",
										Image: "gsoci.azurecr.io/giantswarm/alpine:latest",
										Args:  []string{"sleep", "99999999"},
										SecurityContext: &corev1.SecurityContext{
											AllowPrivilegeEscalation: &f,
											Capabilities: &corev1.Capabilities{
												Drop: []corev1.Capability{"ALL"},
											},
										},
									},
								},
							},
						}
						err := mcClient.Create(ctx, pod)
						if err != nil && !apierrors.IsAlreadyExists(err) {
							Expect(err).NotTo(HaveOccurred())
						}
						DeferCleanup(func() {
							_ = mcClient.Delete(context.Background(), pod)
						})

						Eventually(func() (bool, error) {
							var p corev1.Pod
							if err := mcClient.Get(ctx, types.NamespacedName{Namespace: "default", Name: podName}, &p); err != nil {
								return false, nil //nolint:nilerr
							}
							return p.Status.Phase == corev1.PodRunning, nil
						}).WithPolling(5*time.Second).WithTimeout(2*time.Minute).Should(BeTrue())

						promQL := fmt.Sprintf(`kafka_server_replicamanager_leadercount{cluster_id=%q,namespace=%q}`,
							state.GetCluster().Name, kafkaNamespace)
						Eventually(queryMimirViaPod(ctx, mcClient, podName, promQL)).
							WithPolling(30*time.Second).WithTimeout(5*time.Minute).Should(BeTrue(),
							"PromQL %q returned no results in Mimir after 5 minutes", promQL)
					})
				})
			})
		}).
		Run(t, "strimzi-kafka-operator basic")
}

func kafkaNodePoolManifest() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Version: "v1",
		Kind:    "KafkaNodePool",
	})
	u.SetName(kafkaPoolName)
	u.SetNamespace(kafkaNamespace)
	u.SetLabels(map[string]string{"strimzi.io/cluster": kafkaClusterName})
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

type mimirResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []json.RawMessage `json:"result"`
	} `json:"data"`
}

func queryMimirViaPod(ctx context.Context, mcClient *crclient.Client, podName, promQL string) func() (bool, error) {
	return func() (bool, error) {
		cmd := []string{
			"wget", "-O-", "-Y", "off",
			"--header", "X-Scope-OrgID: giantswarm",
			fmt.Sprintf("%s?query=%s", mimirEndpoint, url.QueryEscape(promQL)),
		}
		stdout, _, err := mcClient.ExecInPod(ctx, podName, "default", "test", cmd)
		if err != nil {
			return false, nil //nolint:nilerr
		}
		var result mimirResponse
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			return false, nil //nolint:nilerr
		}
		return result.Status == "success" && len(result.Data.Result) > 0, nil
	}
}
