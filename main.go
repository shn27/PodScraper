package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
	ctrl "sigs.k8s.io/controller-runtime"
)

var targetPods = map[string][]string{
	"ace": {
		"ace-db-", "inbox-server-jmap", "inbox-ui-",
	},
	"monitoring": {
		"inbox-agent-operator-", "inbox-agent-webhook",
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

	// Infinite loop with 1-minute intervals
	for {
		err := scrapeAndSave(clientset, metricsClient)
		if err != nil {
			log.Printf("Error during scrape: %v", err)
		}
		time.Sleep(1 * time.Minute)
	}
}

func scrapeAndSave(clientset *kubernetes.Clientset, metricsClient *metrics.Clientset) error {
	ctx := context.Background()
	var sb strings.Builder

	sb.WriteString("=========================\n")
	sb.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("Data:\n")

	// Pod Metrics
	sb.WriteString("=== Pod Metrics ===\n")
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pod := range podMetrics.Items {
		if isTargetPod(pod.Namespace, pod.Name) {
			for _, container := range pod.Containers {
				sb.WriteString(fmt.Sprintf("Namespace: %s, Pod: %s, Container: %s, CPU: %s, Memory: %s\n",
					pod.Namespace, pod.Name, container.Name,
					container.Usage.Cpu().String(), container.Usage.Memory().String()))
			}
		}
	}

	// Node Metrics
	sb.WriteString("\n=== Node Metrics ===\n")
	nodeMetrics, err := metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, node := range nodeMetrics.Items {
		sb.WriteString(fmt.Sprintf("Node: %s, CPU: %s, Memory: %s\n",
			node.Name, node.Usage.Cpu().String(), node.Usage.Memory().String()))
	}

	// Pod Status
	sb.WriteString("\n=== All Pods ===\n")
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		if isTargetPod(pod.Namespace, pod.Name) {
			sb.WriteString(fmt.Sprintf("Namespace: %s, Pod: %s, Status: %s\n",
				pod.Namespace, pod.Name, pod.Status.Phase))
		}
	}

	sb.WriteString("=========================\n\n")

	// Append to file
	file, err := os.OpenFile("metrics-log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(sb.String())

	return err
}
