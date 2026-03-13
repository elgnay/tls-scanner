# TLS Analyzer Tool

## Overview

The `tls-analyzer` is a companion tool to `tls-scanner` that transforms and analyzes scan results. It separates the analysis/transformation logic from the scanning process.

## Philosophy

**Separation of Concerns:**
- **tls-scanner** - Performs network scanning and collects data
- **tls-analyzer** - Transforms and analyzes collected data

**Benefits:**
- Analyze results offline without rescanning
- Run multiple analyses on the same scan data
- Faster iteration on analysis logic
- Easier testing and validation

## Installation

Built alongside the scanner:

```bash
make build
```

Output:
```
bin/tls-scanner   # Scanner binary
bin/tls-analyzer  # Analyzer binary
```

## Usage

### Basic Analysis

```bash
# Analyze JSON results
tls-analyzer --input results.json
```

Output: Summary printed to stdout

### Group by Deployment

Group results by deployment instead of individual pods:

```bash
tls-analyzer --input results.json --by-deployment
```

**Use case:** Clusters with multiple replicas where you want to see deployment-level TLS configuration rather than per-pod details.

### Compact View

Show one row per component with all ports listed:

```bash
tls-analyzer --input results.json --by-deployment --compact
```

**Example output:**
```
POD/DEPLOYMENT                               PORTS
--------------------------------------------------------------------------------
cluster-manager                              8443(https)
cluster-proxy                                8090(https), 8091(https), 8092(http), 8095(unknown)
ocm-controller                               8000(http), 8080(http)
ocm-webhook                                  8000(https)
```

**Use case:** Quick overview of all services and their ports in a single compact table.

### Export Transformed Results

```bash
# Export to JSON
tls-analyzer --input results.json \
  --by-deployment \
  --output-json merged-results.json

# Analyze without merging
tls-analyzer --input results.json \
  --output-json analyzed-results.json
```

## Command-Line Options

| Option | Description | Required |
|--------|-------------|----------|
| `--input <file>` | Input JSON file from tls-scanner | Yes |
| `--by-deployment` | Group results by deployment name | No |
| `--service <types>` | Filter by service types, comma-separated (e.g., https or http) | No |
| `--compact` | Show compact view: one row per component with all ports | No |
| `--show-namespace` | Show namespace column in compact view | No |
| `--output-json <file>` | Output transformed JSON | No |
| `--version` | Print version and exit | No |

## Workflows

### Workflow 1: Scan → Analyze → Export

```bash
# Step 1: Scan the cluster
tls-scanner --all-pods \
  --json-file scan-results.json \
  --log-file scan.log

# Step 2: Analyze and merge by deployment
tls-analyzer --input scan-results.json \
  --by-deployment \
  --output-json deployment-summary.json
```

### Workflow 2: Filter Then Analyze

```bash
# Step 1: Scan with filters
tls-scanner --all-pods \
  --namespace-filter "production,staging" \
  --json-file filtered-scan.json

# Step 2: Analyze and show only TLS-enabled ports
tls-analyzer --input filtered-scan.json \
  --by-deployment \
  --service https

# Step 3: Save TLS-enabled results
tls-analyzer --input filtered-scan.json \
  --by-deployment \
  --service https \
  --output-json tls-only.json
```

### Workflow 3: Multiple Analyses

```bash
# Single scan
tls-scanner --all-pods --json-file scan.json

# Analysis 1: View all results
tls-analyzer --input scan.json

# Analysis 2: View only HTTPS ports (TLS-enabled)
tls-analyzer --input scan.json --service https

# Analysis 3: View only HTTP ports (non-TLS)
tls-analyzer --input scan.json --service http

# Analysis 4: Save per-deployment TLS-enabled results
tls-analyzer --input scan.json \
  --by-deployment \
  --service https \
  --output-json tls-deployments.json

# Analysis 7: Multiple output formats
tls-analyzer --input scan.json --output-json per-pod.json
tls-analyzer --input scan.json --by-deployment --output-json per-deployment.json
```

## Group by Deployment

### How It Works

1. **Groups pods** by their owner (Deployment, StatefulSet, DaemonSet, ReplicaSet)
2. **Aggregates IPs** from all replicas
3. **Merges port results** - combines unique TLS versions and ciphers
4. **Replaces pod name** with deployment name

### Example

**Input (per-pod):**
```json
{
  "ip_results": [
    {
      "ip": "10.128.0.15",
      "pod": {"name": "kube-apiserver-master-0"},
      "port_results": [{
        "port": 6443,
        "tls_versions": ["TLSv1.2", "TLSv1.3"],
        "tls_ciphers": ["TLS_AES_128_GCM_SHA256"]
      }]
    },
    {
      "ip": "10.128.0.16",
      "pod": {"name": "kube-apiserver-master-1"},
      "port_results": [{
        "port": 6443,
        "tls_versions": ["TLSv1.2", "TLSv1.3"],
        "tls_ciphers": ["TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"]
      }]
    }
  ]
}
```

**Output (per-deployment):**
```json
{
  "ip_results": [
    {
      "ip": "10.128.0.15,10.128.0.16",
      "pod": {
        "name": "kube-apiserver",
        "ips": ["10.128.0.15", "10.128.0.16"]
      },
      "port_results": [{
        "port": 6443,
        "tls_versions": ["TLSv1.2", "TLSv1.3"],
        "tls_ciphers": [
          "TLS_AES_128_GCM_SHA256",
          "TLS_AES_256_GCM_SHA384"
        ]
      }]
    }
  ]
}
```

### When to Use Merge

**Use merge when:**
- ✅ You want deployment-level compliance reporting
- ✅ Cluster has many replicas (reduces output size)
- ✅ Per-pod differences are not important
- ✅ You need cleaner reports for management

**Don't use merge when:**
- ❌ Investigating pod-specific TLS issues
- ❌ Debugging individual pod configurations
- ❌ You need to know which specific pod has issues
- ❌ Tracking differences between replicas

## Output Formats

### JSON Output

Full scan results in JSON format:

```bash
tls-analyzer --input scan.json --output-json result.json
```

**Use for:**
- Programmatic processing
- Further analysis with jq
- Storage and archival
- API integration
- Input to other tools

### stdout (Default)

When no output file is specified, results are printed to stdout in a table format:

```bash
tls-analyzer --input scan.json --by-deployment
```

**Output format:**
- Table with columns: Pod/Deployment, Port, Service, TLS Versions, TLS Ciphers
- One row per port
- Shows deployment name when merged by deployment
- Truncates long names and cipher lists for readability

**Example output:**
```
=== CLUSTER SCAN RESULTS ===
Timestamp: 2024-01-15T10:30:00Z
Total IPs: 150
Successfully Scanned: 145

POD/DEPLOYMENT                                     PORT   SERVICE    TLS VERSIONS             TLS CIPHERS
---------------------------------------------------------------------------------------------------------------------------------------------------------------------
kube-apiserver                                     6443   https      TLSv1.2, TLSv1.3         TLS_AES_256_GCM_SHA384, TLS_CHACHA20_POLY1305_SHA256... (6)
prometheus-k8s-0                                   9090   http
etcd-master-0                                      2379   https      TLSv1.2, TLSv1.3         TLS_AES_128_GCM_SHA256, TLS_CHACHA20_POLY1305_SHA256, ECDHE-RSA-AES256...
```

**Use for:**
- Quick inspection
- Piping to other commands
- Interactive analysis
- Development and testing

**Note:** For CSV and JUnit XML output, use the scanner directly:
```bash
# CSV output from scanner
tls-scanner --all-pods --csv-file results.csv

# JUnit output from scanner
tls-scanner --all-pods --junit-file results.xml
```

## Programmatic Usage

### With jq

```bash
# Find all HTTP ports in merged results
tls-analyzer --input scan.json \
  --by-deployment \
  --output-json merged.json

jq '.ip_results[] |
    select(.port_results[]? | .service == "http") |
    {deployment: .pod.name, namespace: .pod.namespace}' \
    merged.json
```

### In Shell Scripts

```bash
#!/bin/bash

# Scan
tls-scanner --all-pods --json-file scan.json

# Analyze
tls-analyzer --input scan.json \
  --by-deployment \
  --output-csv deployments.csv

# Check for failures
if grep -q "NO_TLS" deployments.csv; then
    echo "WARNING: Found HTTP ports"
fi
```

### In CI/CD Pipeline

```yaml
# GitLab CI example
scan-tls:
  script:
    - tls-scanner --all-pods --json-file scan.json
    - tls-analyzer --input scan.json --output-junit results.xml
  artifacts:
    reports:
      junit: results.xml
```

## Performance

**Analyzer is fast** - processes results in memory:

| Operation | Time (typical) |
|-----------|---------------|
| Load 1000 pods | ~100ms |
| Merge by deployment | ~50ms |
| Export CSV | ~200ms |
| **Total** | **< 1 second** |

**No network I/O** - pure data transformation.

## Future Enhancements

Potential features:
- [ ] Filter transformations (exclude namespaces, deployments)
- [ ] Custom grouping (by namespace, by component type)
- [ ] Statistical analysis (TLS version distribution)
- [ ] Compliance scoring
- [ ] Diff between scan results
- [ ] HTML report generation

## See Also

- [Scanner Documentation](../README.md)
- [Merge Implementation](../internal/scanner/merge.go)
- [Output Formats](../internal/output/)
