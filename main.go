package main

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
	"log"
	"os"
	ctrl "sigs.k8s.io/controller-runtime"
	"strconv"
	"strings"
	"time"
)

func convertKiToMi(s string) (int64, error) {
	if !strings.HasSuffix(s, "Ki") {
		return 0, fmt.Errorf("invalid unit, expected 'Ki'")
	}

	valueStr := strings.TrimSuffix(s, "Ki")
	valueKi, err := strconv.ParseInt(valueStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse value %s : %w", s, err)
	}

	valueMi := valueKi / 1024
	return valueMi, nil
}

var targetPods = map[string][]string{
	"ace": {
		"ace-db-", "inbox-server-jmap", "inbox-ui-",
	},
	"monitoring": {
		"inbox-agent-operator", "inbox-agent-webhook-",
	},
}

func isTargetPod(namespace, podName string) bool {
	for _, prefix := range targetPods[namespace] {
		if strings.HasPrefix(podName, prefix) {
			return true
		}
	}
	return false
}

var podMetricsCPUData map[string]int64
var podMetricsMemoryData map[string]int64
var sb strings.Builder

func main() {
	config, err := ctrl.GetConfig()
	if err != nil {
		config, err = restclient.InClusterConfig()
		if err != nil {
			panic(err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating kubernetes client: %v", err)
	}

	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating metrics client: %v", err)
	}

	podMetricsCPUData = make(map[string]int64)
	podMetricsMemoryData = make(map[string]int64)

	// Infinite loop with 1-minute intervals
	for i := 0; i < 30; i++ {
		err := scrapeAndSave(clientset, metricsClient)
		if err != nil {
			log.Printf("Error during scrape: %v", err)
		}
		time.Sleep(1 * time.Minute)
	}
	sb.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("Data:\n")
	sb.WriteString("\n===============================================Pod Metrics Maximum CPU Usage===================================\n")
	for key, value := range podMetricsCPUData {
		sb.WriteString(fmt.Sprintf("Pod Name: %s Maximum CPU usage: %v\n", key, value))
	}
	sb.WriteString("\n===============================================Pod Metrics Maximum Memory Usage===================================\n")

	for key, value := range podMetricsMemoryData {
		sb.WriteString(fmt.Sprintf("Pod Name: %s Maximum Memory usage: %v Mi\n", key, value))
	}

	// Node Metrics
	sb.WriteString("\n===============================================Node Metrics Data======================================\n")
	nodeMetrics, err := metricsClient.MetricsV1beta1().NodeMetricses().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, node := range nodeMetrics.Items {
		memory, err := convertKiToMi(node.Usage.Memory().String())
		if err != nil {
			return
		}
		sb.WriteString(fmt.Sprintf("Node: %s, CPU: %s, Memory: %vMi\n",
			node.Name, node.Usage.Cpu().String(), memory))
	}

	// Append to file
	file, err := os.OpenFile("metrics-log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer file.Close()
	_, err = file.WriteString(sb.String())
}

func scrapeAndSave(clientset *kubernetes.Clientset, metricsClient *metrics.Clientset) error {
	ctx := context.Background()

	// Pod Metrics
	//sb.WriteString("=== Pod Metrics ===\n")
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pod := range podMetrics.Items {
		if isTargetPod(pod.Namespace, pod.Name) {
			for _, container := range pod.Containers {
				memory, err := convertKiToMi(container.Usage.Memory().String())
				if err != nil {
					continue
				}

				//sb.WriteString(fmt.Sprintf("Namespace: %s, Pod: %s, Container: %s, CPU: %s, Memory: %vMi\n",
				//	pod.Namespace, pod.Name, container.Name,
				//	container.Usage.Cpu().String(), memory))

				podMetricsCPUData[pod.Namespace+"/"+pod.Name+"/"+container.Name] = max(podMetricsCPUData[pod.Namespace+"/"+pod.Name], container.Usage.Cpu().Value()) // todo
				podMetricsMemoryData[pod.Namespace+"/"+pod.Name+"/"+container.Name] = max(podMetricsCPUData[pod.Namespace+"/"+pod.Name], memory)
			}
		}
	}

	// Pod Status
	//sb.WriteString("\n=== All Pods ===\n")
	//pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	//if err != nil {
	//	return err
	//}
	//for _, pod := range pods.Items {
	//	if isTargetPod(pod.Namespace, pod.Name) {
	//		sb.WriteString(fmt.Sprintf("Namespace: %s, Pod: %s, Status: %s\n",
	//			pod.Namespace, pod.Name, pod.Status.Phase))
	//	}
	//}
	//
	//sb.WriteString("=========================\n\n")

	// Append to file
	file, err := os.OpenFile("metrics-log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(sb.String())

	return err
}
