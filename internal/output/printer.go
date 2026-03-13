package output

import (
	"fmt"
	"log"
	"strings"

	"github.com/openshift/tls-scanner/internal/scanner"
)

func PrintClusterResults(results scanner.ScanResults) {
	fmt.Printf("=== CLUSTER SCAN RESULTS ===\n")
	fmt.Printf("Timestamp: %s\n", results.Timestamp)
	fmt.Printf("Total IPs: %d\n", results.TotalIPs)
	fmt.Printf("Successfully Scanned: %d\n\n", results.ScannedIPs)

	// Print table header
	fmt.Printf("%-50s %-6s %-10s %-25s %-70s\n",
		"POD/DEPLOYMENT", "PORT", "SERVICE", "TLS VERSIONS", "TLS CIPHERS")
	fmt.Printf("%s\n", strings.Repeat("-", 165))

	for _, ipResult := range results.IPResults {
		// Determine pod/deployment name
		podName := ""
		if ipResult.Pod != nil {
			podName = ipResult.Pod.Name
		}

		// Skip entries with errors
		if ipResult.Error != "" {
			fmt.Printf("%-50s %-6s %-10s %-25s %s\n",
				truncateMiddle(podName, 50),
				"-",
				"ERROR",
				"",
				truncateString(ipResult.Error, 70))
			continue
		}

		// Print each port result
		for _, portResult := range ipResult.PortResults {
			if portResult.Error != "" {
				fmt.Printf("%-50s %-6d %-10s %-25s %s\n",
					truncateMiddle(podName, 50),
					portResult.Port,
					"ERROR",
					"",
					truncateString(portResult.Error, 70))
				continue
			}

			// Format TLS versions
			tlsVersions := ""
			if len(portResult.TlsVersions) > 0 {
				tlsVersions = strings.Join(portResult.TlsVersions, ", ")
			}

			// Format TLS ciphers (show count if too many)
			// Try TlsCiphers first, fall back to TlsKeyExchange.ForwardSecrecy.ECDHE
			var cipherList []string
			if len(portResult.TlsCiphers) > 0 {
				cipherList = portResult.TlsCiphers
			} else if portResult.TlsKeyExchange != nil &&
				portResult.TlsKeyExchange.ForwardSecrecy != nil &&
				len(portResult.TlsKeyExchange.ForwardSecrecy.ECDHE) > 0 {
				cipherList = portResult.TlsKeyExchange.ForwardSecrecy.ECDHE
			}

			tlsCiphers := ""
			if len(cipherList) > 0 {
				if len(cipherList) <= 3 {
					tlsCiphers = strings.Join(cipherList, ", ")
				} else {
					tlsCiphers = fmt.Sprintf("%s... (%d)",
						strings.Join(cipherList[:2], ", "),
						len(cipherList))
				}
			}

			fmt.Printf("%-50s %-6d %-10s %-25s %s\n",
				truncateMiddle(podName, 50),
				portResult.Port,
				portResult.Service,
				truncateString(tlsVersions, 25),
				truncateString(tlsCiphers, 70))
		}
	}

	fmt.Printf("\n")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func truncateMiddle(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 6 {
		return s[:maxLen]
	}
	// Split the available space between start and end
	// Reserve 3 chars for "..."
	availableSpace := maxLen - 3
	startLen := availableSpace / 2
	endLen := availableSpace - startLen

	return s[:startLen] + "..." + s[len(s)-endLen:]
}

// PrintCompactView prints results in compact format: one row per component with all ports
func PrintCompactView(results scanner.ScanResults, showNamespace bool) {
	fmt.Printf("=== CLUSTER SCAN RESULTS (COMPACT VIEW) ===\n")
	fmt.Printf("Timestamp: %s\n", results.Timestamp)
	fmt.Printf("Total IPs: %d\n", results.TotalIPs)
	fmt.Printf("Successfully Scanned: %d\n\n", results.ScannedIPs)

	// Print table header
	if showNamespace {
		fmt.Printf("%-40s %-50s %s\n",
			"NAMESPACE", "POD/DEPLOYMENT", "PORTS")
		fmt.Printf("%s\n", strings.Repeat("-", 160))
	} else {
		fmt.Printf("%-50s %s\n",
			"POD/DEPLOYMENT", "PORTS")
		fmt.Printf("%s\n", strings.Repeat("-", 120))
	}

	for _, ipResult := range results.IPResults {
		// Determine namespace and pod/deployment name
		namespace := ""
		podName := ""
		if ipResult.Pod != nil {
			namespace = ipResult.Pod.Namespace
			podName = ipResult.Pod.Name
		}

		// Skip entries with errors
		if ipResult.Error != "" {
			if showNamespace {
				fmt.Printf("%-40s %-50s %s\n",
					truncateMiddle(namespace, 40),
					truncateMiddle(podName, 50),
					"ERROR: "+truncateString(ipResult.Error, 65))
			} else {
				fmt.Printf("%-50s %s\n",
					truncateMiddle(podName, 50),
					"ERROR: "+truncateString(ipResult.Error, 65))
			}
			continue
		}

		// Build port list string
		var portList []string
		for _, portResult := range ipResult.PortResults {
			if portResult.Error != "" {
				portList = append(portList, fmt.Sprintf("%d(error)", portResult.Port))
			} else {
				portList = append(portList, fmt.Sprintf("%d(%s)", portResult.Port, portResult.Service))
			}
		}

		portsStr := strings.Join(portList, ", ")
		if showNamespace {
			fmt.Printf("%-40s %-50s %s\n",
				truncateMiddle(namespace, 40),
				truncateMiddle(podName, 50),
				portsStr)
		} else {
			fmt.Printf("%-50s %s\n",
				truncateMiddle(podName, 50),
				portsStr)
		}
	}

	fmt.Printf("\n")
}

func formatNamespacePod(namespace, podName string, maxLen int) string {
	fullName := fmt.Sprintf("%s/%s", namespace, podName)
	if len(fullName) <= maxLen {
		return fullName
	}

	// Reserve 1 char for "/"
	availableSpace := maxLen - 1

	// Split available space between namespace and pod name
	// Give slightly more space to pod name (60% pod, 40% namespace)
	namespaceMax := (availableSpace * 2) / 5
	podMax := availableSpace - namespaceMax

	truncatedNamespace := truncateMiddle(namespace, namespaceMax)
	truncatedPod := truncateMiddle(podName, podMax)

	return fmt.Sprintf("%s/%s", truncatedNamespace, truncatedPod)
}

func PrintParsedResults(results scanner.ScanResults) {
	if len(results.IPResults) == 0 {
		log.Println("No hosts were scanned or host is down.")
		return
	}

	for _, ipResult := range results.IPResults {
		for _, portResult := range ipResult.PortResults {
			fmt.Printf("PORT    STATE SERVICE REASON\n")
			fmt.Printf("%d/%s %-5s %-7s %s\n", portResult.Port, portResult.Protocol, portResult.State, portResult.Service, portResult.Reason)

			if len(portResult.TlsVersions) > 0 || len(portResult.TlsCiphers) > 0 {
				fmt.Println("| ssl-enum-ciphers:")
				for _, version := range portResult.TlsVersions {
					fmt.Printf("|   %s:\n", version)
				}
				if len(portResult.TlsCiphers) > 0 {
					fmt.Printf("|   ciphers:\n")
					for _, cipher := range portResult.TlsCiphers {
						strength := portResult.TlsCipherStrength[cipher]
						if strength != "" {
							fmt.Printf("|     %s - %s\n", cipher, strength)
						} else {
							fmt.Printf("|     %s\n", cipher)
						}
					}
				}
			}
		}
	}
}

func PrintPQCClusterResults(results scanner.ScanResults) {
	fmt.Printf("\n========================================\n")
	fmt.Printf("PQC CHECK RESULTS\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Timestamp: %s\n", results.Timestamp)
	fmt.Printf("Total IPs: %d\n", results.TotalIPs)
	fmt.Printf("Scanned:   %d\n", results.ScannedIPs)
	fmt.Printf("\n")

	tls13Count := 0
	mlkemCount := 0
	pqcReadyCount := 0

	for _, ipResult := range results.IPResults {
		fmt.Printf("-----------------------------------------------------\n")
		fmt.Printf("IP: %s\n", ipResult.IP)

		if ipResult.Pod != nil {
			fmt.Printf("Pod: %s/%s\n", ipResult.Pod.Namespace, ipResult.Pod.Name)
		}
		if ipResult.OpenshiftComponent != nil {
			fmt.Printf("Component: %s\n", ipResult.OpenshiftComponent.Component)
		}

		if ipResult.Error != "" {
			fmt.Printf("  Error: %s\n", ipResult.Error)
			continue
		}

		for _, portResult := range ipResult.PortResults {
			if portResult.Status == scanner.StatusNoPorts {
				fmt.Printf("  No TCP ports declared\n")
				continue
			}

			fmt.Printf("  Port %d:\n", portResult.Port)

			if portResult.TLS13Supported {
				fmt.Printf("    TLS 1.3:  SUPPORTED\n")
				tls13Count++
			} else {
				fmt.Printf("    TLS 1.3:  NOT SUPPORTED\n")
			}

			if portResult.MLKEMSupported {
				fmt.Printf("    ML-KEM:   SUPPORTED\n")
				fmt.Printf("    ML-KEM KEMs: %s\n", strings.Join(portResult.MLKEMCiphers, ", "))
				mlkemCount++
			} else {
				fmt.Printf("    ML-KEM:   NOT SUPPORTED\n")
			}

			if portResult.TLS13Supported && portResult.MLKEMSupported {
				pqcReadyCount++
			}

			if len(portResult.TlsVersions) > 0 {
				fmt.Printf("    TLS Versions: %s\n", strings.Join(portResult.TlsVersions, ", "))
			}

			if len(portResult.AllKEMs) > 0 {
				fmt.Printf("    All KEMs: %s\n", strings.Join(portResult.AllKEMs, ", "))
			}
		}
	}

	fmt.Printf("\n========================================\n")
	fmt.Printf("SUMMARY\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Total Ports Scanned: %d\n", results.ScannedIPs)
	fmt.Printf("TLS 1.3 Ready:       %d\n", tls13Count)
	fmt.Printf("ML-KEM Ready:        %d\n", mlkemCount)
	fmt.Printf("Fully PQC Ready:     %d (TLS 1.3 + ML-KEM)\n", pqcReadyCount)
	fmt.Printf("========================================\n")
}
