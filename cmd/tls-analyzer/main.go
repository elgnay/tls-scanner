package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/openshift/tls-scanner/internal/output"
	"github.com/openshift/tls-scanner/internal/scanner"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("tls-analyzer", flag.ContinueOnError)

	inputFile := fs.String("input", "", "Input JSON file from tls-scanner (required)")
	outputJSON := fs.String("output-json", "", "Output JSON file (optional)")
	byDeployment := fs.Bool("by-deployment", false, "Group scan results by deployment name")
	serviceFilter := fs.String("service", "", "Filter by service type - comma-separated (e.g., https,http)")
	compactView := fs.Bool("compact", false, "Show compact view: one row per component with all ports")
	showNamespace := fs.Bool("show-namespace", false, "Show namespace column in compact view")
	showVersion := fs.Bool("version", false, "Print version and exit")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *showVersion {
		fmt.Println("tls-analyzer v1.0.0")
		return 0
	}

	if *inputFile == "" {
		fmt.Println("Error: --input is required")
		fmt.Println("\nUsage: tls-analyzer --input results.json [options]")
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
		return 1
	}

	// Read input JSON file
	data, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Printf("Error reading input file %s: %v", *inputFile, err)
		return 1
	}

	// Parse scan results
	var results scanner.ScanResults
	if err := json.Unmarshal(data, &results); err != nil {
		log.Printf("Error parsing JSON from %s: %v", *inputFile, err)
		return 1
	}

	log.Printf("Loaded scan results from %s", *inputFile)
	log.Printf("  Timestamp: %s", results.Timestamp)
	log.Printf("  Total IPs: %d", results.TotalIPs)
	log.Printf("  Scanned IPs: %d", results.ScannedIPs)
	log.Printf("  IP Results: %d", len(results.IPResults))

	// Apply transformations
	if *byDeployment {
		log.Println("Grouping results by deployment...")
		results = scanner.MergeResultsByDeployment(results)
		log.Printf("After grouping: %d deployment results", len(results.IPResults))
	}

	// Apply filters
	if *serviceFilter != "" {
		log.Printf("Filtering to show only %s ports...", *serviceFilter)
		results = filterByService(results, *serviceFilter)
		log.Printf("After filter: %d results with %s ports", len(results.IPResults), *serviceFilter)
	}

	// Write outputs
	if *outputJSON != "" {
		if err := writeJSON(results, *outputJSON); err != nil {
			log.Printf("Error writing JSON output: %v", err)
			return 1
		}
		log.Printf("JSON output written to: %s", *outputJSON)
	}

	// Print summary to stdout if no output file specified
	if *outputJSON == "" {
		if *compactView {
			output.PrintCompactView(results, *showNamespace)
		} else {
			output.PrintClusterResults(results)
		}
	}

	return 0
}

func writeJSON(results scanner.ScanResults, filename string) error {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filename, data, 0644)
}

func filterByService(results scanner.ScanResults, serviceFilter string) scanner.ScanResults {
	// Parse comma-separated service types
	serviceTypes := make(map[string]bool)
	for _, s := range strings.Split(serviceFilter, ",") {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			serviceTypes[trimmed] = true
		}
	}

	var filtered []scanner.IPResult

	for _, ipResult := range results.IPResults {
		// Filter port results to only include specified services
		var matchingPortResults []scanner.PortResult
		for _, portResult := range ipResult.PortResults {
			if serviceTypes[portResult.Service] {
				matchingPortResults = append(matchingPortResults, portResult)
			}
		}

		// Only include IPResult if it has at least one matching port
		if len(matchingPortResults) > 0 {
			ipResult.PortResults = matchingPortResults
			filtered = append(filtered, ipResult)
		}
	}

	results.IPResults = filtered
	return results
}
