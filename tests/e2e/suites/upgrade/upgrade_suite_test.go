package upgrade_test

import (
	"fmt"
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
)

func TestUpgrade(t *testing.T) {
	var wcClient *crclient.Client
	var kafkaNodePoolRef *unstructured.Unstructured
	var kafkaRef *unstructured.Unstructured
	var namespaceRef *corev1.Namespace

	suite.New().
		WithHelmRelease(true).
		WithHelmTargetNamespace(installNamespace).
		WithIsUpgrade(true).
		WithValuesFile("./values.yaml").
		AfterClusterReady(func() {
			It("should have WC API connectivity", func() {
				var err error
				wcClient, err = state.GetFramework().WC(state.GetCluster().Name)
				Expect(err).NotTo(HaveOccurred())
			})
		}).
		BeforeUpgrade(func() {
			// Deploy a Kafka cluster before the upgrade so we can verify it
			// survives the operator rolling update.
			It("operator Deployment should be ready before upgrade", func() {
				ctx := state.GetContext()
				var err error
				wcClient, err = state.GetFramework().WC(state.GetCluster().Name)
				Expect(err).NotTo(HaveOccurred())

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
					"operator Deployment not ready before upgrade")
			})

			It("Kafka cluster should be ready before upgrade", func() {
				ctx := state.GetContext()
				var err error
				wcClient, err = state.GetFramework().WC(state.GetCluster().Name)
				Expect(err).NotTo(HaveOccurred())

				namespaceRef = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: kafkaNamespace},
				}
				err = wcClient.Create(ctx, namespaceRef)
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
					"Kafka broker pod not ready before upgrade")
			})
		}).
		Tests(func() {
			// Post-upgrade checks: verify the operator and Kafka cluster survived.
			Describe("post-upgrade", Ordered, func() {
				BeforeAll(func() {
					var err error
					wcClient, err = state.GetFramework().WC(state.GetCluster().Name)
					Expect(err).NotTo(HaveOccurred())
					if namespaceRef == nil {
						namespaceRef = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: kafkaNamespace}}
					}
					if kafkaNodePoolRef == nil {
						kafkaNodePoolRef = kafkaNodePoolManifest()
					}
					if kafkaRef == nil {
						kafkaRef = kafkaManifest()
					}
				})

				AfterAll(func() {
					ctx := state.GetContext()
					_ = wcClient.Delete(ctx, namespaceRef)
					Eventually(wait.IsResourceDeleted(ctx, wcClient, namespaceRef)).
						WithPolling(10 * time.Second).WithTimeout(5 * time.Minute).Should(BeTrue())
				})

				It("operator Deployment should be ready after upgrade", func() {
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
						"operator Deployment not ready after upgrade")
				})

				It("Kafka CRs should still exist after upgrade", func() {
					ctx := state.GetContext()
					Eventually(wait.DoesResourceExist(ctx, wcClient, kafkaNodePoolRef)).
						WithPolling(5 * time.Second).WithTimeout(1 * time.Minute).Should(BeTrue(),
						"KafkaNodePool CR missing after upgrade")
					Eventually(wait.DoesResourceExist(ctx, wcClient, kafkaRef)).
						WithPolling(5 * time.Second).WithTimeout(1 * time.Minute).Should(BeTrue(),
						"Kafka CR missing after upgrade")
				})

				It("Kafka broker pod should still be ready after upgrade", func() {
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
					}).WithPolling(15*time.Second).WithTimeout(5*time.Minute).Should(BeTrue(),
						"Kafka broker pod not ready after upgrade")
				})
			})
		}).
		Run(t, "strimzi-kafka-operator upgrade")
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
