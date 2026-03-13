# tls-scanner

A TLS compliance scanner for OpenShift/Kubernetes clusters. Uses [testssl.sh](https://testssl.sh/) to enumerate TLS versions, cipher suites, and key exchange groups across cluster pods, with post-quantum (ML-KEM) readiness checks.

## Prerequisites

- **Go environment** - For building the scanner binary.
- **Container tool** - Docker or Podman for building and pushing the scanner image.
- **OpenShift/Kubernetes cluster access** - `oc` or `kubectl` configured to point to your target cluster.
- **Sufficient privileges** - Permissions to create Jobs, and grant `cluster-reader` and `privileged` SCC to a ServiceAccount.

## Installation & Usage

The scanner is designed to be run from within the cluster it is scanning. This is the most reliable way to ensure network access to all pods. The included `deploy.sh` script automates the build and deployment process.

### CI/CD Workflow (Recommended)

This is the recommended approach for automated scanning in an ephemeral test environment.

#### 1. Configure Environment

Set these environment variables in your CI job:

- `SCANNER_IMAGE`: The full tag of the image to build and push (e.g., `quay.io/my-org/tls-scanner:latest`).
- `NAMESPACE`: The OpenShift/Kubernetes namespace where the scanner Job will be deployed (e.g., `scanner-project`). **Note:** This does NOT restrict what gets scanned - it only determines where the scanner runs.
- `NAMESPACE_FILTER`: (Optional) Comma-separated list of namespaces to scan. If not set, the scanner will scan all pods in the entire cluster. Example: `production,staging` to scan only those namespaces.
- `DEPLOYMENT_FILTER`: (Optional) Comma-separated list of deployment names to scan. If not set, all pods are scanned (subject to namespace filter). Example: `kube-apiserver,etcd` to scan only those deployments.
- `KUBECONFIG`: Path to the kubeconfig file for the ephemeral test cluster.
- `JOB_TEMPLATE_FILE`: (Optional) Path to the Job manifest template used by `deploy.sh deploy` (default: `scanner-job.yaml.template`). For host-based scanning, use a template that runs with host network access (e.g. `scanner-job-microshift.yaml.template`).
- `SCAN_MODE`: (Optional) `pod` (default) or `host`. **Pod mode** discovers pods in the cluster and scans their TLS ports. **Host mode** is for environments where core components (API server, etcd, kubelet) run on the host rather than in pods—e.g. single-node or edge setups such as MicroShift. Use a job template with `hostNetwork: true` and set `SCAN_MODE=host` so the scanner runs on the host and can reach those services.

#### 2. Build and Push the Image

Your CI pipeline needs to be authenticated with your container registry.

```bash
# Build the binary and container image
./deploy.sh build

# Push the image to your registry
./deploy.sh push
```

#### 3. Deploy the Scan Job

This step creates the necessary RBAC permissions and deploys a Kubernetes Job into the ephemeral cluster.

```bash
./deploy.sh deploy
```

#### 4. Wait for Completion and Collect Results

The CI job must wait for the Kubernetes Job to complete and then copy the artifacts out.

```bash
# Wait for the job to finish (adjust timeout as needed)
kubectl wait --for=condition=complete job/tls-scanner-job -n "$NAMESPACE" --timeout=15m

# Get the name of the pod created by the job
POD_NAME=$(kubectl get pods -n "$NAMESPACE" -l job-name=tls-scanner-job -o jsonpath='{.items[0].metadata.name}')

# Create a local directory for artifacts
mkdir -p ./artifacts

# Copy all result files from the pod
kubectl cp "${NAMESPACE}/${POD_NAME}:/artifacts/." "./artifacts/"
```

Your `./artifacts` directory will now contain `results.json`, `results.csv`, and `scan.log`.

### Ignoring Deployments

You can exclude specific deployments from scanning by creating a `.tlsscannerignore` file in your working directory. This is useful for:
- Skipping test/development deployments
- Excluding known-good deployments to reduce scan time
- Filtering out third-party components

#### Creating an Ignore File

Create a `.tlsscannerignore` file in your current directory:

```bash
# .tlsscannerignore
# Lines starting with # are comments
# One deployment name per line

# Ignore by deployment name only (applies to all namespaces)
test-deployment
dev-app

# Ignore by namespace/deployment (more specific)
kube-system/coredns
monitoring/prometheus-operator
```

#### Using the Ignore File

**Default behavior** (automatically reads `.tlsscannerignore`):
```bash
./tls-scanner --all-pods
```

**Custom ignore file location**:
```bash
./tls-scanner --all-pods --ignore-file /path/to/my-ignores.txt
```

**Disable ignore file**:
```bash
./tls-scanner --all-pods --ignore-file ""
```

#### Automatic Exclusions

The scanner automatically excludes certain types of pods:
- **Job pods**: Pods owned by Kubernetes Jobs are automatically excluded from scanning since they are typically temporary/transient workloads.

This happens automatically and does not require any configuration.

#### Ignore File Format

- **One deployment per line**
- **Comments**: Lines starting with `#` are ignored
- **Empty lines**: Ignored
- **Formats supported**:
  - `deployment-name` - Matches deployment in any namespace
  - `namespace/deployment-name` - Matches specific namespace/deployment combination

**Example**:
```
# System components to skip
kube-system/kube-dns
kube-system/kube-proxy

# Development deployments
dev-
test-app
staging-frontend

# Third-party operators
openshift-operators/cert-manager
```

## Analyzing Scan Results

After running a scan, you can use the `tls-analyzer` tool to transform and analyze the JSON results.

### Installation

The analyzer is built alongside the scanner:

```bash
make build  # Builds both tls-scanner and tls-analyzer
```

### Grouping Results by Deployment

When scanning clusters with multiple replicas of the same deployment, you can group results per deployment:

```bash
# Analyze and merge by deployment
tls-analyzer --input artifacts/results.json \
  --by-deployment \
  --output-json merged-results.json
```

**What it does:**
- Groups all pods belonging to the same deployment (Deployment, StatefulSet, DaemonSet)
- Aggregates all IPs from the replicas into a single result entry
- Combines port scan results, merging unique TLS versions and ciphers found across all replicas
- Shows the deployment name instead of individual pod names

**Example output difference:**

**Without grouping** (per-pod):
```csv
IP,Port,Pod Name,Namespace,TLS Ciphers
10.128.0.15,8443,kube-apiserver-master-0,openshift-kube-apiserver,TLS_AES_128_GCM_SHA256
10.128.0.16,8443,kube-apiserver-master-1,openshift-kube-apiserver,TLS_AES_128_GCM_SHA256
10.128.0.17,8443,kube-apiserver-master-2,openshift-kube-apiserver,TLS_AES_128_GCM_SHA256
```

**With grouping** (per-deployment):
```csv
IP,Port,Pod Name,Namespace,TLS Ciphers
"10.128.0.15,10.128.0.16,10.128.0.17",8443,kube-apiserver,openshift-kube-apiserver,TLS_AES_128_GCM_SHA256
```

### Analyzer Options

```bash
tls-analyzer [OPTIONS]
```

**Options:**
- `--input <file>` - Input JSON file from tls-scanner (required)
- `--by-deployment` - Group scan results by deployment name
- `--service <types>` - Filter by service types, comma-separated (e.g., https or http)
- `--compact` - Show compact view: one row per component with all ports
- `--show-namespace` - Show namespace column in compact view
- `--output-json <file>` - Output transformed JSON to file
- `--version` - Print version and exit

### Usage Examples

```bash
# View results as a table (stdout)
tls-analyzer --input results.json --by-deployment

# Show compact view: one row per component with all ports
tls-analyzer --input results.json --by-deployment --compact

# Show compact view with namespace column
tls-analyzer --input results.json --by-deployment --compact --show-namespace

# Show only HTTPS ports (TLS-enabled)
tls-analyzer --input results.json --service https

# Show only HTTP ports (non-TLS)
tls-analyzer --input results.json --service http

# Combine filters: group by deployment and show only TLS-enabled services in compact view
tls-analyzer --input results.json \
  --by-deployment \
  --service https \
  --compact

# Save transformed results to JSON
tls-analyzer --input results.json \
  --by-deployment \
  --output-json merged-results.json

# Analyze without merging
tls-analyzer --input artifacts/results.json \
  --output-json analyzed-results.json
```

**Table output:** When no output file is specified, the analyzer prints a table showing each port with its service type, TLS versions, and ciphers. This makes it easy to quickly scan for HTTP vs HTTPS ports and verify TLS configurations.

### Host-based scanning

In some environments, core cluster components (API server, etcd, kubelet) run on the host instead of as pods. Examples include single-node and edge deployments such as MicroShift. For those setups, use **host** scan mode so the scanner runs with host network access and can discover and scan TLS services bound to the host:

```bash
JOB_TEMPLATE_FILE=scanner-job-microshift.yaml.template SCAN_MODE=host ./deploy.sh deploy
```

Use `JOB_TEMPLATE_FILE` to point at a job template that sets `spec.hostNetwork: true` (and any required security context). The default `scanner-job.yaml.template` is for pod-based scanning only.

## Understanding Scan Results

### CSV Output Columns

The scanner produces a CSV file with detailed information about each scanned port. Key columns include:

| Column             | Description                                                 |
| ------------------ | ----------------------------------------------------------- |
| IP                 | Pod IP address that was scanned                             |
| Port               | TCP port number                                             |
| Protocol           | Protocol (typically "tcp")                                  |
| Service            | Detected service (e.g., "https", "http")                    |
| Pod Name           | Kubernetes pod name                                         |
| Namespace          | Kubernetes namespace                                        |
| Component Name     | OpenShift component name extracted from image               |
| Process            | Process name listening on the port (from lsof)              |
| TLS Ciphers        | Detected TLS cipher suites                                  |
| TLS Version        | Detected TLS versions (e.g., TLSv1.2, TLSv1.3)              |
| **Status**         | Categorized scan result (see below)                         |
| **Reason**         | Detailed explanation of the status                          |
| **Listen Address** | Address the port is bound to (e.g., 127.0.0.1, \*, 0.0.0.0) |

### Status Categories

The `Status` column categorizes why a port couldn't be scanned or its TLS configuration:

| Status           | Description                                                    |
| ---------------- | -------------------------------------------------------------- |
| `OK`             | TLS scan successful - cipher and version information available |
| `NO_TLS`         | Port is open but not using TLS (plain HTTP/TCP service)        |
| `LOCALHOST_ONLY` | Port is bound to 127.0.0.1, not accessible from pod IP         |
| `FILTERED`       | Port blocked by network policy or firewall                     |
| `CLOSED`         | Port not listening on the scanned IP address                   |
| `MTLS_REQUIRED`  | TLS handshake failed - likely requires client certificate      |
| `TIMEOUT`        | Connection timed out                                           |
| `NO_PORTS`       | Pod declares no TCP ports in its spec                          |
| `ERROR`          | Scan error occurred (see Reason for details)                   |

### Service Detection

The scanner automatically detects the service type for each port:

| Service  | Description                                                          |
| -------- | -------------------------------------------------------------------- |
| `https`  | TLS/SSL detected - port is serving encrypted traffic                |
| `http`   | Plain HTTP detected - port is serving unencrypted HTTP traffic      |
| `unknown`| Port is open but service type could not be determined               |

**Detection methods:**
- **TLS services** (`https`): Detected by testssl.sh TLS handshake
- **Plain HTTP** (`http`): Detected by sending HTTP/1.0 GET request and checking response
- **Unknown**: Port open but doesn't respond to HTTP or has no TLS

**Note:** Ports with `service: "http"` and `status: "NO_TLS"` are typically:
- Metrics endpoints (Prometheus)
- Health/readiness probes
- Internal service communication in trusted networks
- These are **expected and acceptable** when not exposed externally

### Example Output

```csv
IP,Port,Protocol,Service,Pod Name,Namespace,...,Status,Reason,Listen Address
10.128.0.15,8443,tcp,https,kube-apiserver-pod,openshift-kube-apiserver,...,OK,TLS scan successful,*
10.0.53.147,9257,tcp,unknown,controller-manager-pod,openshift-cloud-controller,...,LOCALHOST_ONLY,Bound to 127.0.0.1 not accessible from pod IP,127.0.0.1
10.0.87.193,10258,tcp,unknown,aws-ccm-pod,openshift-cloud-controller,...,FILTERED,Network policy or firewall blocking access,N/A
10.128.0.20,8080,tcp,http,metrics-pod,openshift-monitoring,...,NO_TLS,Port open, serving plain HTTP (no TLS),*
10.128.0.147,8383,tcp,http,klusterlet-addon-controller,open-cluster-management,...,NO_TLS,Port open, serving plain HTTP (no TLS),*
```

### Interpreting Results

1. **LOCALHOST_ONLY ports**: These are internal-only ports bound to 127.0.0.1. They're only accessible from within the pod itself and don't pose external TLS compliance concerns. Common for metrics endpoints that are scraped via sidecar containers.

2. **FILTERED ports**: The scanner couldn't reach these ports due to network policies, firewall rules, or the service not listening on the pod IP. Review network policies if you need to scan these.

3. **NO_TLS ports**: These ports are open but don't use TLS. The `Service` field indicates what protocol was detected:
   - `service: "http"` - Plain HTTP detected (common for metrics, health checks)
   - `service: "unknown"` - Port is open but protocol could not be determined

   This is **expected** for internal endpoints like Prometheus metrics or Kubernetes health probes. These are secure when:
   - Not exposed externally (no LoadBalancer/NodePort/Ingress)
   - Protected by NetworkPolicies
   - Serving non-sensitive data (metrics, status)

4. **MTLS_REQUIRED ports**: Common for etcd (ports 2379/2380) and other services requiring mutual TLS. The scanner can't complete the handshake without a client certificate.

#### 5. Cleanup

Remove the scanner Job and associated RBAC permissions from the cluster.

```bash
./deploy.sh cleanup
```

### Manual Usage

You can also run the steps manually.

1.  **Build the image:** `export SCANNER_IMAGE="your-registry/image:tag"` and run `./deploy.sh build`.
2.  **Push the image:** `./deploy.sh push`.
3.  **Deploy the job:** `export NAMESPACE="scanner-project" NAMESPACE_FILTER="production"` and run `./deploy.sh deploy`
4.  **Monitor and retrieve results** as shown in the CI workflow.
5.  **Clean up** with `./deploy.sh cleanup`.

### `deploy.sh` Script Actions

- `build`: Builds the `tls-scanner` binary and container image.
- `push`: Pushes the container image to the registry specified by `$SCANNER_IMAGE`.
- `deploy`: Deploys the scanner Kubernetes Job to the cluster specified by `$KUBECONFIG` and `$NAMESPACE`.
- `cleanup`: Removes the scanner Job and RBAC resources.
- `full-deploy` (or no action): Runs `build`, `push`, and `deploy`.

### Command Line Options

The scanner binary accepts the following command-line options. These are configured in the `scanner-job.yaml.template` file.

```bash
./tls-scanner [OPTIONS]
```

**Options:**

- `-host <ip>` - Target host/IP to scan (default: 127.0.0.1)
- `-port <port>` - Target port to scan (default: 443)
- `-targets <host:port,...>` - Comma-separated list of host:port targets to scan
- `-all-pods` - Scan all pods in the cluster (requires cluster access)
- `-component-filter <names>` - Filter pods by component name (comma-separated, used with -all-pods)
- `-deployment-filter <names>` - Filter pods by deployment name (comma-separated, used with -all-pods)
- `-namespace-filter <names>` - Filter pods by namespace (comma-separated, used with -all-pods)
- `-ignore-file <path>` - Path to file containing deployment names to ignore (default: .tlsscannerignore)
- `-limit-ips <num>` - Cap number of IPs to scan, for testing (0 = no limit)
- `-pqc-check` - Check for TLS 1.3 + ML-KEM (post-quantum) support; exits non-zero on failure
- `-j <num>` - Number of concurrent scans; 0 = runtime.NumCPU() (default: 0)
- `-artifact-dir <dir>` - Directory for output files (default: /tmp)
- `-json-file <file>` - Output results in JSON format to specified file
- `-csv-file <file>` - Output results in CSV format to specified file
- `-junit-file <file>` - Output results in JUnit XML format to specified file
- `-log-file <file>` - Redirect all log output to the specified file
- `-timing-file <file>` - Write timing report to specified file in artifact-dir
- `-disable-lsof` - Disable lsof for process discovery (use when lsof is not available in containers)
- `-version` - Print version and exit
