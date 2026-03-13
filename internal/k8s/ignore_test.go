package k8s

import (
	"os"
	"path/filepath"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLoadIgnoreFile(t *testing.T) {
	// Create a temporary ignore file
	tmpDir := t.TempDir()
	ignoreFile := filepath.Join(tmpDir, ".tlsscannerignore")

	content := `# This is a comment
test-deployment
kube-system/coredns

# Another comment
namespace-a/app-b
`
	err := os.WriteFile(ignoreFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test ignore file: %v", err)
	}

	ignoreSet, err := LoadIgnoreFile(ignoreFile)
	if err != nil {
		t.Fatalf("LoadIgnoreFile failed: %v", err)
	}

	expected := map[string]struct{}{
		"test-deployment":      {},
		"kube-system/coredns":  {},
		"namespace-a/app-b":    {},
	}

	if len(ignoreSet) != len(expected) {
		t.Errorf("Expected %d entries, got %d", len(expected), len(ignoreSet))
	}

	for key := range expected {
		if _, ok := ignoreSet[key]; !ok {
			t.Errorf("Expected to find %s in ignore set", key)
		}
	}
}

func TestLoadIgnoreFile_NotExists(t *testing.T) {
	ignoreSet, err := LoadIgnoreFile("/nonexistent/path/.tlsscannerignore")
	if err != nil {
		t.Fatalf("Should not error when file doesn't exist: %v", err)
	}

	if len(ignoreSet) != 0 {
		t.Errorf("Expected empty ignore set, got %d entries", len(ignoreSet))
	}
}

func TestFilterPodsFromIgnoreList(t *testing.T) {
	pods := []PodInfo{
		{
			Name:      "test-deployment-abc123",
			Namespace: "default",
			Pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment-abc123",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "ReplicaSet", Name: "test-deployment-abc123"},
					},
				},
			},
		},
		{
			Name:      "production-app-xyz789",
			Namespace: "production",
			Pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "production-app-xyz789",
					Namespace: "production",
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "ReplicaSet", Name: "production-app-xyz789"},
					},
				},
			},
		},
		{
			Name:      "coredns-123",
			Namespace: "kube-system",
			Pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "coredns-123",
					Namespace: "kube-system",
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "ReplicaSet", Name: "coredns-123"},
					},
				},
			},
		},
	}

	ignoreSet := map[string]struct{}{
		"test-deployment":      {}, // Should match first pod
		"kube-system/coredns":  {}, // Should match third pod by namespace/deployment
	}

	filtered := FilterPodsFromIgnoreList(pods, ignoreSet)

	// Should only keep production-app
	if len(filtered) != 1 {
		t.Errorf("Expected 1 pod after filtering, got %d", len(filtered))
	}

	if len(filtered) > 0 && filtered[0].Namespace != "production" {
		t.Errorf("Expected production namespace, got %s", filtered[0].Namespace)
	}
}

func TestFilterPodsFromIgnoreList_EmptyIgnoreSet(t *testing.T) {
	pods := []PodInfo{
		{Name: "pod1", Namespace: "default"},
		{Name: "pod2", Namespace: "default"},
	}

	ignoreSet := map[string]struct{}{}
	filtered := FilterPodsFromIgnoreList(pods, ignoreSet)

	if len(filtered) != len(pods) {
		t.Errorf("Expected all pods to remain with empty ignore set, got %d/%d", len(filtered), len(pods))
	}
}
