package features_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/giantswarm/apptest-framework/v4/pkg/state"
	"github.com/giantswarm/apptest-framework/v4/pkg/suite"
	crclient "github.com/giantswarm/clustertest/v4/pkg/client"
	"github.com/giantswarm/clustertest/v4/pkg/wait"
	cr "sigs.k8s.io/controller-runtime/pkg/client"

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

	// bootstrap address used for exec-in-pod Kafka CLI calls.
	bootstrapAddress = "test-cluster-kafka-bootstrap:9092"
)

func TestFeatures(t *testing.T) {
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
			Describe("strimzi-kafka-operator features", Ordered, func() {
				var wcClient *crclient.Client
				var namespaceRef *corev1.Namespace

				BeforeAll(func() {
					ctx := state.GetContext()

					By("obtaining WC client")
					Eventually(func() error {
						var err error
						wcClient, err = state.GetFramework().WC(state.GetCluster().Name)
						return err
					}).WithPolling(5*time.Second).WithTimeout(2*time.Minute).Should(Succeed(),
						"failed to obtain WC client")

					By("creating test namespace " + kafkaNamespace)
					namespaceRef = &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{Name: kafkaNamespace},
					}
					Eventually(func() error {
						err := wcClient.Create(ctx, namespaceRef)
						if apierrors.IsAlreadyExists(err) {
							return nil
						}
						return err
					}).WithPolling(5*time.Second).WithTimeout(2*time.Minute).Should(Succeed(),
						"failed to create namespace %s", kafkaNamespace)

					By("deploying KafkaNodePool and Kafka CRs")
					pool := kafkaNodePoolManifest()
					Eventually(func() error {
						err := wcClient.Create(ctx, pool)
						if apierrors.IsAlreadyExists(err) {
							return nil
						}
						return err
					}).WithPolling(5*time.Second).WithTimeout(2*time.Minute).Should(Succeed(),
						"failed to create KafkaNodePool")
					kafka := kafkaManifest()
					Eventually(func() error {
						err := wcClient.Create(ctx, kafka)
						if apierrors.IsAlreadyExists(err) {
							return nil
						}
						return err
					}).WithPolling(5*time.Second).WithTimeout(2*time.Minute).Should(Succeed(),
						"failed to create Kafka CR")

					By("waiting for Kafka broker pod to be ready")
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
					}).WithPolling(15*time.Second).WithTimeout(10*time.Minute).Should(BeTrue())
				})

				AfterAll(func() {
					ctx := state.GetContext()
					_ = wcClient.Delete(ctx, namespaceRef)
					Eventually(wait.IsResourceDeleted(ctx, wcClient, namespaceRef)).
						WithPolling(10 * time.Second).WithTimeout(5 * time.Minute).Should(BeTrue())
				})

				// ------------------------------------------------------------
				// Chaos: operator pod deletion and self-healing
				// ------------------------------------------------------------
				It("operator should recover after pod deletion", func() {
					ctx := state.GetContext()

					By("finding the operator pod")
					var podList corev1.PodList
					Expect(wcClient.List(ctx, &podList,
						cr.InNamespace(installNamespace),
						cr.MatchingLabels{"name": operatorDeploymentName},
					)).To(Succeed())
					Expect(podList.Items).NotTo(BeEmpty(), "no operator pods found in %s", installNamespace)
					operatorPod := podList.Items[0]

					By("deleting the operator pod")
					Expect(wcClient.Delete(ctx, &operatorPod)).To(Succeed())

					By("waiting for the operator Deployment to recover")
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
					}).WithPolling(10*time.Second).WithTimeout(3*time.Minute).Should(BeTrue(),
						"operator Deployment did not recover after pod deletion")

					By("verifying the Kafka broker pod is still ready")
					podName := fmt.Sprintf("%s-%s-0", kafkaClusterName, kafkaPoolName)
					var brokerPod corev1.Pod
					Expect(wcClient.Get(ctx, types.NamespacedName{
						Namespace: kafkaNamespace,
						Name:      podName,
					}, &brokerPod)).To(Succeed())
					ready := false
					for _, c := range brokerPod.Status.Conditions {
						if c.Type == corev1.PodReady {
							ready = c.Status == corev1.ConditionTrue
						}
					}
					Expect(ready).To(BeTrue(),
						"Kafka broker pod %s/%s not ready after operator chaos", kafkaNamespace, podName)
				})

				// ------------------------------------------------------------
				// KafkaTopic lifecycle
				// ------------------------------------------------------------
				Describe("KafkaTopic lifecycle", Ordered, func() {
					topicName := "test-topic"
					topicRef := kafkaTopicManifest(topicName)

					AfterAll(func() {
						_ = wcClient.Delete(state.GetContext(), topicRef)
					})

					It("KafkaTopic CR should become Ready", func() {
						ctx := state.GetContext()
						err := wcClient.Create(ctx, topicRef)
						if err != nil && !apierrors.IsAlreadyExists(err) {
							Expect(err).NotTo(HaveOccurred())
						}

						Eventually(func() (bool, error) {
							var t unstructured.Unstructured
							t.SetGroupVersionKind(topicRef.GroupVersionKind())
							if err := wcClient.Get(ctx, types.NamespacedName{
								Namespace: kafkaNamespace,
								Name:      topicName,
							}, &t); err != nil {
								return false, nil //nolint:nilerr
							}
							conditions, _, _ := unstructured.NestedSlice(t.Object, "status", "conditions")
							for _, c := range conditions {
								if m, ok := c.(map[string]interface{}); ok {
									if m["type"] == "Ready" && m["status"] == "True" {
										return true, nil
									}
								}
							}
							return false, nil
						}).WithPolling(10*time.Second).WithTimeout(3*time.Minute).Should(BeTrue(),
							"KafkaTopic %s/%s never became Ready", kafkaNamespace, topicName)
					})

					It("should persist messages across broker pod restart", func() {
						ctx := state.GetContext()
						brokerPodName := fmt.Sprintf("%s-%s-0", kafkaClusterName, kafkaPoolName)
						testMessage := "e2e-persistence-test"

						By("producing a message")
						Eventually(func() error {
							_, _, err := wcClient.ExecInPod(ctx, brokerPodName, kafkaNamespace, "kafka",
								[]string{"sh", "-c", fmt.Sprintf(
									"echo %q | /opt/kafka/bin/kafka-console-producer.sh --bootstrap-server %s --topic %s",
									testMessage, bootstrapAddress, topicName,
								)},
							)
							return err
						}).WithPolling(5*time.Second).WithTimeout(2*time.Minute).Should(Succeed(),
							"failed to produce message to topic %s", topicName)

						By("deleting the broker pod to trigger a restart")
						brokerPod := &corev1.Pod{}
						Expect(wcClient.Get(ctx, types.NamespacedName{
							Namespace: kafkaNamespace,
							Name:      brokerPodName,
						}, brokerPod)).To(Succeed())
						Expect(wcClient.Delete(ctx, brokerPod)).To(Succeed())

						By("waiting for the broker pod to be ready again")
						Eventually(func() (bool, error) {
							var pod corev1.Pod
							if err := wcClient.Get(ctx, types.NamespacedName{
								Namespace: kafkaNamespace,
								Name:      brokerPodName,
							}, &pod); err != nil {
								return false, nil //nolint:nilerr
							}
							for _, c := range pod.Status.Conditions {
								if c.Type == corev1.PodReady {
									return c.Status == corev1.ConditionTrue, nil
								}
							}
							return false, nil
						}).WithPolling(10*time.Second).WithTimeout(5*time.Minute).Should(BeTrue(),
							"broker pod did not recover after restart")

						By("consuming the message and verifying it survived the restart")
						Eventually(func() (bool, error) {
							stdout, _, err := wcClient.ExecInPod(ctx, brokerPodName, kafkaNamespace, "kafka",
								[]string{
									"/opt/kafka/bin/kafka-console-consumer.sh",
									"--bootstrap-server", bootstrapAddress,
									"--topic", topicName,
									"--from-beginning",
									"--max-messages", "1",
									"--timeout-ms", "10000",
								},
							)
							if err != nil {
								return false, nil //nolint:nilerr
							}
							return strings.Contains(stdout, testMessage), nil
						}).WithPolling(15*time.Second).WithTimeout(2*time.Minute).Should(BeTrue(),
							"message %q not found in topic %s after broker restart", testMessage, topicName)
					})
				})

				// ------------------------------------------------------------
				// KafkaUser + User Operator
				// ------------------------------------------------------------
				Describe("KafkaUser lifecycle", Ordered, func() {
					userName := "test-user"
					userRef := kafkaUserManifest(userName)

					AfterAll(func() {
						_ = wcClient.Delete(state.GetContext(), userRef)
					})

					It("KafkaUser CR should be reconciled and Secret created", func() {
						ctx := state.GetContext()
						err := wcClient.Create(ctx, userRef)
						if err != nil && !apierrors.IsAlreadyExists(err) {
							Expect(err).NotTo(HaveOccurred())
						}

						// The User Operator creates a Secret with the same name as the KafkaUser.
						Eventually(func() error {
							var secret corev1.Secret
							return wcClient.Get(ctx, types.NamespacedName{
								Namespace: kafkaNamespace,
								Name:      userName,
							}, &secret)
						}).WithPolling(10*time.Second).WithTimeout(3*time.Minute).Should(Succeed(),
							"Secret for KafkaUser %s/%s was not created", kafkaNamespace, userName)
					})

					It("KafkaUser CR should become Ready", func() {
						ctx := state.GetContext()
						Eventually(func() (bool, error) {
							var u unstructured.Unstructured
							u.SetGroupVersionKind(userRef.GroupVersionKind())
							if err := wcClient.Get(ctx, types.NamespacedName{
								Namespace: kafkaNamespace,
								Name:      userName,
							}, &u); err != nil {
								return false, nil //nolint:nilerr
							}
							conditions, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
							for _, c := range conditions {
								if m, ok := c.(map[string]interface{}); ok {
									if m["type"] == "Ready" && m["status"] == "True" {
										return true, nil
									}
								}
							}
							return false, nil
						}).WithPolling(10*time.Second).WithTimeout(3*time.Minute).Should(BeTrue(),
							"KafkaUser %s/%s never became Ready", kafkaNamespace, userName)
					})
				})

				// ------------------------------------------------------------
				// Topic replication factor enforcement
				// ------------------------------------------------------------
				Describe("topic replication enforcement", Ordered, func() {
					validTopicName   := "valid-rf-topic"
					invalidTopicName := "invalid-rf-topic"
					validTopicRef    := kafkaTopicManifestWithRF(validTopicName, 1)
					invalidTopicRef  := kafkaTopicManifestWithRF(invalidTopicName, 3)

					AfterAll(func() {
						_ = wcClient.Delete(state.GetContext(), validTopicRef)
						_ = wcClient.Delete(state.GetContext(), invalidTopicRef)
					})

					It("topic with replicationFactor=1 should become Ready", func() {
						ctx := state.GetContext()
						err := wcClient.Create(ctx, validTopicRef)
						if err != nil && !apierrors.IsAlreadyExists(err) {
							Expect(err).NotTo(HaveOccurred())
						}

						Eventually(func() (bool, error) {
							var t unstructured.Unstructured
							t.SetGroupVersionKind(validTopicRef.GroupVersionKind())
							if err := wcClient.Get(ctx, types.NamespacedName{
								Namespace: kafkaNamespace,
								Name:      validTopicName,
							}, &t); err != nil {
								return false, nil //nolint:nilerr
							}
							conditions, _, _ := unstructured.NestedSlice(t.Object, "status", "conditions")
							for _, c := range conditions {
								if m, ok := c.(map[string]interface{}); ok {
									if m["type"] == "Ready" && m["status"] == "True" {
										return true, nil
									}
								}
							}
							return false, nil
						}).WithPolling(10*time.Second).WithTimeout(3*time.Minute).Should(BeTrue(),
							"topic %s/%s with replicationFactor=1 never became Ready", kafkaNamespace, validTopicName)
					})

					It("topic with replicationFactor=3 should surface an error condition", func() {
						ctx := state.GetContext()
						err := wcClient.Create(ctx, invalidTopicRef)
						if err != nil && !apierrors.IsAlreadyExists(err) {
							Expect(err).NotTo(HaveOccurred())
						}

						// The Topic Operator should set a non-Ready or error condition because
						// replicationFactor=3 exceeds the number of brokers (1).
						Eventually(func() (bool, error) {
							var t unstructured.Unstructured
							t.SetGroupVersionKind(invalidTopicRef.GroupVersionKind())
							if err := wcClient.Get(ctx, types.NamespacedName{
								Namespace: kafkaNamespace,
								Name:      invalidTopicName,
							}, &t); err != nil {
								return false, nil //nolint:nilerr
							}
							conditions, _, _ := unstructured.NestedSlice(t.Object, "status", "conditions")
							for _, c := range conditions {
								if m, ok := c.(map[string]interface{}); ok {
									// Any condition with status != True indicates the operator
									// surfaced the problem (NotReady, Error, etc.)
									if m["type"] == "Ready" && m["status"] != "True" {
										return true, nil
									}
								}
							}
							return false, nil
						}).WithPolling(10*time.Second).WithTimeout(3*time.Minute).Should(BeTrue(),
							"topic with replicationFactor=3 did not surface an error condition")
					})
				})
			})
		}).
		Run(t, "strimzi-kafka-operator features")
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
					"type":          "persistent-claim",
					"size":          "5Gi",
					"kraftMetadata": "shared",
					"deleteClaim":   true,
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

func kafkaTopicManifest(name string) *unstructured.Unstructured {
	return kafkaTopicManifestWithRF(name, 1)
}

func kafkaTopicManifestWithRF(name string, replicationFactor int64) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Version: "v1",
		Kind:    "KafkaTopic",
	})
	u.SetName(name)
	u.SetNamespace(kafkaNamespace)
	u.SetLabels(map[string]string{"strimzi.io/cluster": kafkaClusterName})
	_ = unstructured.SetNestedField(u.Object, map[string]interface{}{
		"partitions":        int64(1),
		"replicas":          replicationFactor,
		"config":            map[string]interface{}{},
	}, "spec")
	return u
}

func kafkaUserManifest(name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "kafka.strimzi.io",
		Version: "v1",
		Kind:    "KafkaUser",
	})
	u.SetName(name)
	u.SetNamespace(kafkaNamespace)
	u.SetLabels(map[string]string{"strimzi.io/cluster": kafkaClusterName})
	_ = unstructured.SetNestedField(u.Object, map[string]interface{}{
		"authentication": map[string]interface{}{
			"type": "scram-sha-512",
		},
	}, "spec")
	return u
}
