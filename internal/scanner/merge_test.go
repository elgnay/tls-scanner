package scanner

import (
	"testing"

	"github.com/openshift/tls-scanner/internal/k8s"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMergeResultsByDeployment(t *testing.T) {
	// Create test data with 3 pods from the same deployment
	replicaSet := "kube-apiserver-abc123"
	deployment := "kube-apiserver"

	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-abc123-pod1",
			Namespace: "openshift-kube-apiserver",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: replicaSet,
				},
			},
		},
		Status: v1.PodStatus{
			PodIP: "10.0.0.1",
		},
	}

	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-abc123-pod2",
			Namespace: "openshift-kube-apiserver",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: replicaSet,
				},
			},
		},
		Status: v1.PodStatus{
			PodIP: "10.0.0.2",
		},
	}

	pod3 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-apiserver-abc123-pod3",
			Namespace: "openshift-kube-apiserver",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: replicaSet,
				},
			},
		},
		Status: v1.PodStatus{
			PodIP: "10.0.0.3",
		},
	}

	results := ScanResults{
		Timestamp:  "2024-01-01T00:00:00Z",
		TotalIPs:   3,
		ScannedIPs: 3,
		IPResults: []IPResult{
			{
				IP:     "10.0.0.1",
				Status: "scanned",
				Pod: &k8s.PodInfo{
					Name:      pod1.Name,
					Namespace: pod1.Namespace,
					IPs:       []string{"10.0.0.1"},
					Pod:       pod1,
				},
				OpenPorts: []int{8443},
				PortResults: []PortResult{
					{
						Port:        8443,
						Protocol:    "tcp",
						State:       "open",
						TlsVersions: []string{"TLSv1.3"},
						TlsCiphers:  []string{"TLS_AES_128_GCM_SHA256"},
						Status:      StatusOK,
					},
				},
			},
			{
				IP:     "10.0.0.2",
				Status: "scanned",
				Pod: &k8s.PodInfo{
					Name:      pod2.Name,
					Namespace: pod2.Namespace,
					IPs:       []string{"10.0.0.2"},
					Pod:       pod2,
				},
				OpenPorts: []int{8443},
				PortResults: []PortResult{
					{
						Port:        8443,
						Protocol:    "tcp",
						State:       "open",
						TlsVersions: []string{"TLSv1.3"},
						TlsCiphers:  []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
						Status:      StatusOK,
					},
				},
			},
			{
				IP:     "10.0.0.3",
				Status: "scanned",
				Pod: &k8s.PodInfo{
					Name:      pod3.Name,
					Namespace: pod3.Namespace,
					IPs:       []string{"10.0.0.3"},
					Pod:       pod3,
				},
				OpenPorts: []int{8443},
				PortResults: []PortResult{
					{
						Port:        8443,
						Protocol:    "tcp",
						State:       "open",
						TlsVersions: []string{"TLSv1.3"},
						TlsCiphers:  []string{"TLS_AES_128_GCM_SHA256"},
						Status:      StatusOK,
					},
				},
			},
		},
	}

	merged := MergeResultsByDeployment(results)

	// Should have 1 merged result instead of 3
	if len(merged.IPResults) != 1 {
		t.Errorf("Expected 1 merged result, got %d", len(merged.IPResults))
	}

	mergedResult := merged.IPResults[0]

	// Check deployment name
	if mergedResult.Pod.Name != deployment {
		t.Errorf("Expected deployment name %s, got %s", deployment, mergedResult.Pod.Name)
	}

	// Check namespace
	if mergedResult.Pod.Namespace != "openshift-kube-apiserver" {
		t.Errorf("Expected namespace openshift-kube-apiserver, got %s", mergedResult.Pod.Namespace)
	}

	// Check that all 3 IPs are included
	if len(mergedResult.Pod.IPs) != 3 {
		t.Errorf("Expected 3 IPs, got %d", len(mergedResult.Pod.IPs))
	}

	// Check that port 8443 is present
	if len(mergedResult.OpenPorts) != 1 || mergedResult.OpenPorts[0] != 8443 {
		t.Errorf("Expected port 8443, got %v", mergedResult.OpenPorts)
	}

	// Check that port results are merged
	if len(mergedResult.PortResults) != 1 {
		t.Errorf("Expected 1 port result, got %d", len(mergedResult.PortResults))
	}

	portResult := mergedResult.PortResults[0]

	// Check TLS versions (should be deduplicated)
	if len(portResult.TlsVersions) != 1 || portResult.TlsVersions[0] != "TLSv1.3" {
		t.Errorf("Expected TLSv1.3, got %v", portResult.TlsVersions)
	}

	// Check TLS ciphers (should include both unique ciphers)
	if len(portResult.TlsCiphers) != 2 {
		t.Errorf("Expected 2 unique ciphers, got %d: %v", len(portResult.TlsCiphers), portResult.TlsCiphers)
	}

	// Check status
	if portResult.Status != StatusOK {
		t.Errorf("Expected status OK, got %s", portResult.Status)
	}
}

func TestMergeResultsByDeployment_DifferentDeployments(t *testing.T) {
	// Create test data with pods from different deployments
	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-a-pod1",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "deployment-a-abc123",
				},
			},
		},
		Status: v1.PodStatus{
			PodIP: "10.0.0.1",
		},
	}

	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "deployment-b-pod1",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind: "ReplicaSet",
					Name: "deployment-b-xyz789",
				},
			},
		},
		Status: v1.PodStatus{
			PodIP: "10.0.0.2",
		},
	}

	results := ScanResults{
		Timestamp:  "2024-01-01T00:00:00Z",
		TotalIPs:   2,
		ScannedIPs: 2,
		IPResults: []IPResult{
			{
				IP:     "10.0.0.1",
				Status: "scanned",
				Pod: &k8s.PodInfo{
					Name:      pod1.Name,
					Namespace: pod1.Namespace,
					IPs:       []string{"10.0.0.1"},
					Pod:       pod1,
				},
				OpenPorts:   []int{8080},
				PortResults: []PortResult{},
			},
			{
				IP:     "10.0.0.2",
				Status: "scanned",
				Pod: &k8s.PodInfo{
					Name:      pod2.Name,
					Namespace: pod2.Namespace,
					IPs:       []string{"10.0.0.2"},
					Pod:       pod2,
				},
				OpenPorts:   []int{8080},
				PortResults: []PortResult{},
			},
		},
	}

	merged := MergeResultsByDeployment(results)

	// Should have 2 separate results (different deployments)
	if len(merged.IPResults) != 2 {
		t.Errorf("Expected 2 merged results, got %d", len(merged.IPResults))
	}
}
