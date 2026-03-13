package scanner

import (
	"log"
	"sort"
	"strings"

	"github.com/openshift/tls-scanner/internal/k8s"
)

// MergeResultsByDeployment aggregates scan results by deployment name
func MergeResultsByDeployment(results ScanResults) ScanResults {
	if len(results.IPResults) == 0 {
		return results
	}

	log.Println("Merging scan results by deployment name...")

	// Group IPResults by deployment name
	deploymentGroups := make(map[string][]IPResult)
	deploymentNamespaces := make(map[string]string) // track primary namespace per deployment

	for _, ipResult := range results.IPResults {
		deploymentName := ""
		namespace := ""

		if ipResult.Pod != nil {
			deploymentName = k8s.GetDeploymentName(*ipResult.Pod)
			namespace = ipResult.Pod.Namespace
		}

		if deploymentName == "" {
			// No deployment info, use IP as key to keep it separate
			deploymentName = "standalone-" + ipResult.IP
		}

		// Use namespace/deployment as key to handle same deployment names in different namespaces
		key := namespace + "/" + deploymentName
		deploymentGroups[key] = append(deploymentGroups[key], ipResult)
		if _, exists := deploymentNamespaces[key]; !exists {
			deploymentNamespaces[key] = namespace
		}
	}

	log.Printf("Grouped %d IP results into %d deployments", len(results.IPResults), len(deploymentGroups))

	// Merge each deployment group
	var mergedResults []IPResult
	for deploymentKey, ipResults := range deploymentGroups {
		if len(ipResults) == 0 {
			continue
		}

		merged := mergeIPResults(deploymentKey, ipResults)
		mergedResults = append(mergedResults, merged)
	}

	// Sort by namespace/deployment name for consistent output
	sort.Slice(mergedResults, func(i, j int) bool {
		nameI := ""
		nameJ := ""
		if mergedResults[i].Pod != nil {
			nameI = mergedResults[i].Pod.Namespace + "/" + mergedResults[i].Pod.Name
		}
		if mergedResults[j].Pod != nil {
			nameJ = mergedResults[j].Pod.Namespace + "/" + mergedResults[j].Pod.Name
		}
		return nameI < nameJ
	})

	results.IPResults = mergedResults
	results.ScannedIPs = len(mergedResults) // Update count to reflect merged results

	log.Printf("Merge complete: %d deployment results", len(mergedResults))

	return results
}

func mergeIPResults(deploymentKey string, ipResults []IPResult) IPResult {
	if len(ipResults) == 0 {
		return IPResult{}
	}

	// Start with the first result as base
	merged := IPResult{
		Status:             ipResults[0].Status,
		OpenshiftComponent: ipResults[0].OpenshiftComponent,
		OpenPorts:          []int{},
		PortResults:        []PortResult{},
	}

	// Collect all IPs from pods in this deployment
	var allIPs []string
	seenIPs := make(map[string]bool)

	// Collect all ports
	portResultsMap := make(map[int][]PortResult)
	seenPorts := make(map[int]bool)

	for _, ipResult := range ipResults {
		// Aggregate IPs
		if !seenIPs[ipResult.IP] {
			allIPs = append(allIPs, ipResult.IP)
			seenIPs[ipResult.IP] = true
		}

		// Aggregate port results
		for _, portResult := range ipResult.PortResults {
			portResultsMap[portResult.Port] = append(portResultsMap[portResult.Port], portResult)
			if !seenPorts[portResult.Port] {
				seenPorts[portResult.Port] = true
				merged.OpenPorts = append(merged.OpenPorts, portResult.Port)
			}
		}
	}

	sort.Ints(merged.OpenPorts)

	// Merge port results for each port
	for _, port := range merged.OpenPorts {
		portResults := portResultsMap[port]
		if len(portResults) == 0 {
			continue
		}

		mergedPort := mergePortResults(portResults)
		merged.PortResults = append(merged.PortResults, mergedPort)
	}

	// Create a synthetic Pod entry representing the deployment
	if len(ipResults) > 0 && ipResults[0].Pod != nil {
		deploymentName := k8s.GetDeploymentName(*ipResults[0].Pod)
		namespace := ipResults[0].Pod.Namespace

		merged.Pod = &k8s.PodInfo{
			Name:      deploymentName,
			Namespace: namespace,
			IPs:       allIPs,
			Image:     ipResults[0].Pod.Image,
			Pod:       ipResults[0].Pod.Pod, // Keep reference to one of the pods (may be nil)
		}
		merged.IP = strings.Join(allIPs, ",")

		log.Printf("Merged deployment %s/%s: %d pods, %d unique IPs, %d unique ports",
			namespace, deploymentName, len(ipResults), len(allIPs), len(merged.OpenPorts))
	} else {
		// Standalone IP without deployment
		merged.IP = ipResults[0].IP
		merged.Pod = ipResults[0].Pod
	}

	return merged
}

func mergePortResults(portResults []PortResult) PortResult {
	if len(portResults) == 0 {
		return PortResult{}
	}

	// Use the first result as base
	merged := portResults[0]

	// Aggregate unique values across all port results
	seenVersions := make(map[string]bool)
	seenCiphers := make(map[string]bool)
	seenProcesses := make(map[string]bool)
	seenContainers := make(map[string]bool)
	seenListenAddrs := make(map[string]bool)

	var allVersions []string
	var allCiphers []string
	var allProcesses []string
	var allContainers []string
	var allListenAddrs []string

	hasOK := false
	hasNoTLS := false

	for _, pr := range portResults {
		// Aggregate TLS versions
		for _, version := range pr.TlsVersions {
			if !seenVersions[version] {
				seenVersions[version] = true
				allVersions = append(allVersions, version)
			}
		}

		// Aggregate TLS ciphers
		for _, cipher := range pr.TlsCiphers {
			if !seenCiphers[cipher] {
				seenCiphers[cipher] = true
				allCiphers = append(allCiphers, cipher)
			}
		}

		// Aggregate process names
		if pr.ProcessName != "" && !seenProcesses[pr.ProcessName] {
			seenProcesses[pr.ProcessName] = true
			allProcesses = append(allProcesses, pr.ProcessName)
		}

		// Aggregate container names
		if pr.ContainerName != "" && !seenContainers[pr.ContainerName] {
			seenContainers[pr.ContainerName] = true
			allContainers = append(allContainers, pr.ContainerName)
		}

		// Aggregate listen addresses
		if pr.ListenAddress != "" && !seenListenAddrs[pr.ListenAddress] {
			seenListenAddrs[pr.ListenAddress] = true
			allListenAddrs = append(allListenAddrs, pr.ListenAddress)
		}

		// Track status
		if pr.Status == StatusOK {
			hasOK = true
		}
		if pr.Status == StatusNoTLS {
			hasNoTLS = true
		}
	}

	// Sort for consistent output
	sort.Strings(allVersions)
	sort.Strings(allCiphers)

	merged.TlsVersions = allVersions
	merged.TlsCiphers = allCiphers
	merged.ProcessName = strings.Join(allProcesses, ",")
	merged.ContainerName = strings.Join(allContainers, ",")
	merged.ListenAddress = strings.Join(allListenAddrs, ",")

	// Set merged status
	if hasOK {
		merged.Status = StatusOK
		merged.Reason = "TLS scan successful (merged from multiple pods)"
	} else if hasNoTLS {
		merged.Status = StatusNoTLS
		merged.Reason = "Port open but no TLS detected (merged from multiple pods)"
	}

	return merged
}
