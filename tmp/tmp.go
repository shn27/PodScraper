package tmp

import (
	"context"
	"fmt"
	"log"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
	ctrl "sigs.k8s.io/controller-runtime"
)

var targetPods = map[string][]string{
	"ace": {
		"ace-db-0", "ace-db-1", "ace-db-2", "inbox-server-jmap", "inbox-ui",
	},
	"monitoring": {
		"inbox-agent-operator", "inbox-agent-webhook",
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

func do() {
	config, err := ctrl.GetConfig()
	if err != nil {
		config, err = restclient.InClusterConfig()
		if err != nil {
			panic(err)
		}
	}

	// Core client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating kubernetes client: %v", err)
	}

	// Metrics client
	metricsClient, err := metrics.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating metrics client: %v", err)
	}

	//// get KB client
	//client, err := GetKBClient()
	//if err != nil {
	//	log.Fatalf("Error creating client: %v", err)
	//}
	//
	ctx := context.Background()
	//
	//client.

	// 1. kubectl top pods --all-namespaces
	fmt.Println("=== Pod Metrics ===")
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error getting pod metrics: %v", err)
	}
	for _, pod := range podMetrics.Items {
		if isTargetPod(pod.Namespace, pod.Name) {
			for _, container := range pod.Containers {
				fmt.Printf("Namespace: %s, Pod: %s, Container: %s, CPU: %s, Memory: %s\n",
					pod.Namespace, pod.Name, container.Name,
					container.Usage.Cpu().String(), container.Usage.Memory().String())
			}
		}
	}

	// 2. kubectl top nodes
	fmt.Println("\n=== Node Metrics ===")
	nodeMetrics, err := metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error getting node metrics: %v", err)
	}
	for _, node := range nodeMetrics.Items {
		fmt.Printf("Node: %s, CPU: %s, Memory: %s\n",
			node.Name, node.Usage.Cpu().String(), node.Usage.Memory().String())
	}

	// 3. kubectl get pods --all-namespaces
	fmt.Println("\n=== All Pods ===")
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Fatalf("Error getting pods: %v", err)
	}
	for _, pod := range pods.Items {
		if isTargetPod(pod.Namespace, pod.Name) {
			fmt.Printf("Namespace: %s, Pod: %s, Status: %s\n",
				pod.Namespace, pod.Name, pod.Status.Phase)
		}
	}
}
