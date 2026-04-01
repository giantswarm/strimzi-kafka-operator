package perf_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/giantswarm/apptest-framework/v4/pkg/state"
	"github.com/giantswarm/apptest-framework/v4/pkg/suite"
	crclient "github.com/giantswarm/clustertest/v4/pkg/client"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const (
	installNamespace = "strimzi-system"
	kafkaNamespace   = "kafka-perf"
	kafkaClusterName = "perf-cluster"
	kafkaPoolName    = "dual-role"
	bootstrapAddress = "perf-cluster-kafka-bootstrap:9092"
	perfTopic        = "perf-test"

	// Minimum acceptable throughput thresholds.
	// These are conservative baselines for a single-broker cluster on shared test infra.
	minProducerMsgPerSec  = 10_000.0
	minProducerMBPerSec   = 5.0
	minConsumerMsgPerSec  = 20_000.0
)

func TestPerf(t *testing.T) {
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
			Describe("strimzi-kafka-operator performance", Ordered, func() {
				var wcClient *crclient.Client
				var namespaceRef *corev1.Namespace

				BeforeAll(func() {
					ctx := state.GetContext()
					var err error
					wcClient, err = state.GetFramework().WC(state.GetCluster().Name)
					Expect(err).NotTo(HaveOccurred())

					By("creating perf namespace " + kafkaNamespace)
					namespaceRef = &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{Name: kafkaNamespace},
					}
					err = wcClient.Create(ctx, namespaceRef)
					if err != nil && !apierrors.IsAlreadyExists(err) {
						Expect(err).NotTo(HaveOccurred())
					}

					By("deploying KafkaNodePool and Kafka CRs")
					pool := kafkaNodePoolManifest()
					err = wcClient.Create(ctx, pool)
					if err != nil && !apierrors.IsAlreadyExists(err) {
						Expect(err).NotTo(HaveOccurred())
					}
					kafka := kafkaManifest()
					err = wcClient.Create(ctx, kafka)
					if err != nil && !apierrors.IsAlreadyExists(err) {
						Expect(err).NotTo(HaveOccurred())
					}

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

					By("creating perf topic")
					topic := kafkaTopicManifest(perfTopic)
					err = wcClient.Create(ctx, topic)
					if err != nil && !apierrors.IsAlreadyExists(err) {
						Expect(err).NotTo(HaveOccurred())
					}
					Eventually(func() (bool, error) {
						var t unstructured.Unstructured
						t.SetGroupVersionKind(topic.GroupVersionKind())
						if err := wcClient.Get(ctx, types.NamespacedName{
							Namespace: kafkaNamespace,
							Name:      perfTopic,
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
					}).WithPolling(10*time.Second).WithTimeout(3*time.Minute).Should(BeTrue())
				})

				AfterAll(func() {
					ctx := state.GetContext()
					_ = wcClient.Delete(ctx, namespaceRef)
				})

				It("producer throughput should meet minimum threshold", func() {
					ctx := state.GetContext()
					brokerPod := fmt.Sprintf("%s-%s-0", kafkaClusterName, kafkaPoolName)

					// 100k messages, 1 KiB each, no throughput cap.
					stdout, _, err := wcClient.ExecInPod(ctx, brokerPod, kafkaNamespace, "kafka",
						[]string{
							"/opt/kafka/bin/kafka-producer-perf-test.sh",
							"--topic", perfTopic,
							"--num-records", "100000",
							"--record-size", "1024",
							"--throughput", "-1",
							"--producer-props", fmt.Sprintf("bootstrap.servers=%s", bootstrapAddress),
						},
					)
					Expect(err).NotTo(HaveOccurred(), "producer perf test failed")

					msgPerSec, mbPerSec := parseProducerOutput(stdout)
					GinkgoLogr.Info("Producer throughput",
						"msg/s", msgPerSec,
						"MB/s", mbPerSec,
					)
					Expect(msgPerSec).To(BeNumerically(">=", minProducerMsgPerSec),
						"producer throughput %.0f msg/s below minimum %.0f", msgPerSec, minProducerMsgPerSec)
					Expect(mbPerSec).To(BeNumerically(">=", minProducerMBPerSec),
						"producer throughput %.2f MB/s below minimum %.2f", mbPerSec, minProducerMBPerSec)
				})

				It("consumer throughput should meet minimum threshold", func() {
					ctx := state.GetContext()
					brokerPod := fmt.Sprintf("%s-%s-0", kafkaClusterName, kafkaPoolName)

					stdout, _, err := wcClient.ExecInPod(ctx, brokerPod, kafkaNamespace, "kafka",
						[]string{
							"/opt/kafka/bin/kafka-consumer-perf-test.sh",
							"--topic", perfTopic,
							"--messages", "100000",
							"--bootstrap-server", bootstrapAddress,
						},
					)
					Expect(err).NotTo(HaveOccurred(), "consumer perf test failed")

					msgPerSec := parseConsumerOutput(stdout)
					GinkgoLogr.Info("Consumer throughput", "msg/s", msgPerSec)
					Expect(msgPerSec).To(BeNumerically(">=", minConsumerMsgPerSec),
						"consumer throughput %.0f msg/s below minimum %.0f", msgPerSec, minConsumerMsgPerSec)
				})
			})
		}).
		Run(t, "strimzi-kafka-operator perf")
}

// parseProducerOutput parses the summary line from kafka-producer-perf-test.sh output.
// Example: "100000 records sent, 45678.9 records/sec (44.61 MB/sec), ..."
func parseProducerOutput(output string) (msgPerSec, mbPerSec float64) {
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, "records sent") {
			continue
		}
		// "45678.9 records/sec"
		if idx := strings.Index(line, "records/sec"); idx > 0 {
			fields := strings.Fields(line[:idx])
			if len(fields) > 0 {
				msgPerSec, _ = strconv.ParseFloat(fields[len(fields)-1], 64)
			}
		}
		// "(44.61 MB/sec)"
		if idx := strings.Index(line, "MB/sec)"); idx > 0 {
			sub := line[:idx]
			fields := strings.Fields(sub)
			if len(fields) > 0 {
				raw := strings.TrimPrefix(fields[len(fields)-1], "(")
				mbPerSec, _ = strconv.ParseFloat(raw, 64)
			}
		}
		return
	}
	return
}

// parseConsumerOutput parses the summary line from kafka-consumer-perf-test.sh output.
// Output is CSV: start, end, data.consumed.in.MB, MB.sec, data.consumed.in.nMsg, nMsg.sec, ...
func parseConsumerOutput(output string) (msgPerSec float64) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		fields := strings.Split(line, ",")
		// Header line starts with "start.time"
		if len(fields) >= 6 && !strings.HasPrefix(strings.TrimSpace(fields[0]), "start") {
			msgPerSec, _ = strconv.ParseFloat(strings.TrimSpace(fields[5]), 64)
			return
		}
	}
	return
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
					"size":          "10Gi",
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
		},
		"entityOperator": map[string]interface{}{
			"topicOperator": map[string]interface{}{},
		},
	}, "spec")
	return u
}

func kafkaTopicManifest(name string) *unstructured.Unstructured {
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
		"partitions": int64(1),
		"replicas":   int64(1),
		"config":     map[string]interface{}{},
	}, "spec")
	return u
}
