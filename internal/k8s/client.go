package k8s

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	configclientset "github.com/openshift/client-go/config/clientset/versioned"
	mcfgclientset "github.com/openshift/client-go/machineconfiguration/clientset/versioned"
	operatorclientset "github.com/openshift/client-go/operator/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func NewClient(disableLsof bool) (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Could not load in-cluster config, falling back to kubeconfig: %v", err)
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		config, err = clientcmd.BuildConfigFromFlags("", loadingRules.GetDefaultFilename())
		if err != nil {
			return nil, fmt.Errorf("could not get kubernetes config: %v", err)
		}
		log.Println("Successfully created Kubernetes client from kubeconfig file")
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	configClient, err := configclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create openshift config client: %v", err)
	}

	operatorClient, err := operatorclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create openshift operator client: %v", err)
	}

	mcfgClient, err := mcfgclientset.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create openshift machineconfig client: %v", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create dynamic client: %v", err)
	}

	namespace := "default"
	if nsBytes, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		namespace = string(nsBytes)
	}

	return &Client{
		clientset:                 clientset,
		restCfg:                   config,
		dynamicClient:             dynamicClient,
		processNameMap:            make(map[string]map[int]string),
		listenInfoMap:             make(map[string]map[int]ListenInfo),
		processDiscoveryAttempted: make(map[string]bool),
		namespace:                 namespace,
		configClient:              configClient,
		operatorClient:            operatorClient,
		mcfgClient:                mcfgClient,
		disableLsof:               disableLsof,
	}, nil
}

func (c *Client) GetAllPodsInfo() []PodInfo {
	log.Println("Getting all pods from the cluster...")
	pods, err := c.clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Printf("Warning: Could not list pods: %v", err)
		return nil
	}

	var allPodsInfo []PodInfo
	for _, pod := range pods.Items {
		if pod.Status.PodIP == "" {
			log.Printf("Skipping pod %s/%s: no IP address assigned (phase: %s)", pod.Namespace, pod.Name, pod.Status.Phase)
			continue
		}

		containerNames := make([]string, 0, len(pod.Spec.Containers))
		for _, container := range pod.Spec.Containers {
			containerNames = append(containerNames, container.Name)
		}

		image := ""
		if len(pod.Spec.Containers) > 0 {
			image = pod.Spec.Containers[0].Image
		}

		podInfo := PodInfo{
			Name:       pod.Name,
			Namespace:  pod.Namespace,
			IPs:        []string{pod.Status.PodIP},
			Image:      image,
			Containers: containerNames,
			Pod:        &pod,
		}
		allPodsInfo = append(allPodsInfo, podInfo)
	}
	log.Printf("Found %d pods in the cluster (with IP addresses)", len(allPodsInfo))

	totalIPs := 0
	uniqueIPs := make(map[string]bool)
	for _, pod := range allPodsInfo {
		for _, ip := range pod.IPs {
			totalIPs++
			uniqueIPs[ip] = true
		}
	}
	log.Printf("IP discovery summary: %d total IPs across %d pods (%d unique IPs).", totalIPs, len(allPodsInfo), len(uniqueIPs))

	return allPodsInfo
}

func (c *Client) FilterPodsByComponent(pods []PodInfo, componentFilter string) []PodInfo {
	if componentFilter == "" {
		return pods
	}

	log.Printf("Filtering pods by component name(s): %s", componentFilter)
	filterComponents := strings.Split(componentFilter, ",")
	filterSet := make(map[string]struct{})
	for _, comp := range filterComponents {
		filterSet[strings.TrimSpace(comp)] = struct{}{}
	}

	var filtered []PodInfo
	for _, pod := range pods {
		component, err := c.GetOpenshiftComponentFromImage(pod.Image)
		if err != nil {
			log.Printf("Warning: could not get component for image %s: %v", pod.Image, err)
			continue
		}
		if _, ok := filterSet[component.Component]; ok {
			filtered = append(filtered, pod)
		}
	}
	log.Printf("Filtered pods: %d remaining out of %d", len(filtered), len(pods))
	return filtered
}

func FilterPodsByNamespace(pods []PodInfo, namespaceFilter string) []PodInfo {
	if namespaceFilter == "" {
		return pods
	}

	log.Printf("Filtering pods by namespace(s): %s", namespaceFilter)
	filterNamespaces := strings.Split(namespaceFilter, ",")
	filterSet := make(map[string]struct{})
	for _, ns := range filterNamespaces {
		filterSet[strings.TrimSpace(ns)] = struct{}{}
	}

	var filtered []PodInfo
	for _, pod := range pods {
		if _, ok := filterSet[pod.Namespace]; ok {
			filtered = append(filtered, pod)
		}
	}
	log.Printf("Filtered pods by namespace: %d remaining out of %d", len(filtered), len(pods))
	return filtered
}

func GetDeploymentName(pod PodInfo) string {
	// If Pod object is available, use owner references
	if pod.Pod != nil {
		// Check owner references for deployment/replicaset
		for _, owner := range pod.Pod.OwnerReferences {
			if owner.Kind == "ReplicaSet" {
				// Extract deployment name from replicaset (format: deployment-name-xxxxx)
				rsName := owner.Name
				// Remove the replicaset hash suffix
				if idx := strings.LastIndex(rsName, "-"); idx != -1 {
					return rsName[:idx]
				}
				return rsName
			}
			if owner.Kind == "Deployment" {
				return owner.Name
			}
			if owner.Kind == "StatefulSet" || owner.Kind == "DaemonSet" {
				return owner.Name
			}
		}

		// Fallback to common labels
		labels := pod.Pod.Labels
		if name, exists := labels["app.kubernetes.io/name"]; exists {
			return name
		}
		if name, exists := labels["app"]; exists {
			return name
		}
	}

	// Extract deployment name from pod name pattern
	// Pod names follow: deployment-name-replicaset-hash-pod-hash
	// Examples:
	//   cluster-manager-56859fc769-94mss -> cluster-manager
	//   ocm-controller-5b5f78cbc4-8cgvk -> ocm-controller
	//   klusterlet-765778b8cd-svvlp -> klusterlet
	podName := pod.Name

	// Remove the last two hash segments
	// First remove pod hash (last segment after last -)
	if idx := strings.LastIndex(podName, "-"); idx != -1 {
		podName = podName[:idx]
	}
	// Then remove replicaset hash (second to last segment)
	if idx := strings.LastIndex(podName, "-"); idx != -1 {
		return podName[:idx]
	}

	// If pattern doesn't match, return the pod name as-is
	return pod.Name
}

func FilterPodsByDeployment(pods []PodInfo, deploymentFilter string) []PodInfo {
	if deploymentFilter == "" {
		return pods
	}

	log.Printf("Filtering pods by deployment name(s): %s", deploymentFilter)
	filterNames := strings.Split(deploymentFilter, ",")
	filterSet := make(map[string]struct{})
	for _, name := range filterNames {
		filterSet[strings.TrimSpace(name)] = struct{}{}
	}

	var filtered []PodInfo
	for _, pod := range pods {
		deploymentName := GetDeploymentName(pod)
		if _, ok := filterSet[deploymentName]; ok {
			filtered = append(filtered, pod)
			log.Printf("Matched pod %s/%s with deployment name: %s", pod.Namespace, pod.Name, deploymentName)
		}
	}
	log.Printf("Filtered pods by deployment: %d remaining out of %d", len(filtered), len(pods))
	return filtered
}

// LoadIgnoreFile reads a .tlsscannerignore file and returns a set of deployment names to ignore
func LoadIgnoreFile(ignoreFilePath string) (map[string]struct{}, error) {
	ignoreSet := make(map[string]struct{})

	// If no path provided, try default location
	if ignoreFilePath == "" {
		ignoreFilePath = ".tlsscannerignore"
	}

	// Check if file exists
	if _, err := os.Stat(ignoreFilePath); os.IsNotExist(err) {
		log.Printf("No ignore file found at %s, proceeding without deployment ignores", ignoreFilePath)
		return ignoreSet, nil
	}

	absPath, _ := filepath.Abs(ignoreFilePath)
	log.Printf("Loading deployment ignore list from: %s", absPath)

	file, err := os.Open(ignoreFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open ignore file %s: %w", ignoreFilePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		ignoreSet[line] = struct{}{}
		log.Printf("  [line %d] Ignoring deployment: %s", lineNum, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading ignore file: %w", err)
	}

	if len(ignoreSet) > 0 {
		log.Printf("Loaded %d deployment(s) to ignore from %s", len(ignoreSet), ignoreFilePath)
	}

	return ignoreSet, nil
}

// FilterPodsFromIgnoreList removes pods whose deployments are in the ignore list
func FilterPodsFromIgnoreList(pods []PodInfo, ignoreSet map[string]struct{}) []PodInfo {
	if len(ignoreSet) == 0 {
		return pods
	}

	log.Printf("Applying deployment ignore list (%d entries)", len(ignoreSet))
	var filtered []PodInfo
	ignoredCount := 0

	for _, pod := range pods {
		deploymentName := GetDeploymentName(pod)
		namespace := pod.Namespace

		// Check both "namespace/deployment" and just "deployment"
		fullName := namespace + "/" + deploymentName
		shouldIgnore := false

		if _, ok := ignoreSet[fullName]; ok {
			shouldIgnore = true
			log.Printf("Ignoring pod %s/%s (matches ignore pattern: %s)", namespace, pod.Name, fullName)
		} else if _, ok := ignoreSet[deploymentName]; ok {
			shouldIgnore = true
			log.Printf("Ignoring pod %s/%s (matches ignore pattern: %s)", namespace, pod.Name, deploymentName)
		}

		if !shouldIgnore {
			filtered = append(filtered, pod)
		} else {
			ignoredCount++
		}
	}

	log.Printf("Filtered out %d pods from ignore list: %d remaining out of %d", ignoredCount, len(filtered), len(pods))
	return filtered
}

// ExcludeJobPods filters out pods that are owned by Kubernetes Jobs
func ExcludeJobPods(pods []PodInfo) []PodInfo {
	log.Println("Excluding pods owned by Jobs...")
	var filtered []PodInfo
	excludedCount := 0

	for _, pod := range pods {
		if pod.Pod == nil {
			filtered = append(filtered, pod)
			continue
		}

		isJobPod := false
		for _, owner := range pod.Pod.OwnerReferences {
			if owner.Kind == "Job" {
				isJobPod = true
				log.Printf("Excluding Job pod: %s/%s (owned by Job: %s)", pod.Namespace, pod.Name, owner.Name)
				break
			}
		}

		if !isJobPod {
			filtered = append(filtered, pod)
		} else {
			excludedCount++
		}
	}

	log.Printf("Excluded %d Job pods: %d remaining out of %d", excludedCount, len(filtered), len(pods))
	return filtered
}
