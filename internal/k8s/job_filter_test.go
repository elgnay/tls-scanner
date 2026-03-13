package k8s

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExcludeJobPods(t *testing.T) {
	trueVal := true

	tests := []struct {
		name     string
		pods     []PodInfo
		expected int
	}{
		{
			name: "exclude job pods",
			pods: []PodInfo{
				{
					Name:      "job-pod-xyz",
					Namespace: "default",
					IPs:       []string{"10.0.0.1"},
					Pod: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-pod-xyz",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "Job",
									Name: "my-job",
									Controller: &trueVal,
								},
							},
						},
					},
				},
				{
					Name:      "deployment-pod-abc",
					Namespace: "default",
					IPs:       []string{"10.0.0.2"},
					Pod: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "deployment-pod-abc",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "ReplicaSet",
									Name: "my-deployment-xyz",
									Controller: &trueVal,
								},
							},
						},
					},
				},
			},
			expected: 1, // Only non-Job pod should remain
		},
		{
			name: "no job pods",
			pods: []PodInfo{
				{
					Name:      "deployment-pod-1",
					Namespace: "default",
					IPs:       []string{"10.0.0.1"},
					Pod: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "deployment-pod-1",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "ReplicaSet",
									Name: "my-deployment-xyz",
									Controller: &trueVal,
								},
							},
						},
					},
				},
				{
					Name:      "statefulset-pod-0",
					Namespace: "default",
					IPs:       []string{"10.0.0.2"},
					Pod: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "statefulset-pod-0",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "StatefulSet",
									Name: "my-statefulset",
									Controller: &trueVal,
								},
							},
						},
					},
				},
			},
			expected: 2, // Both non-Job pods should remain
		},
		{
			name: "all job pods",
			pods: []PodInfo{
				{
					Name:      "job-pod-1",
					Namespace: "default",
					IPs:       []string{"10.0.0.1"},
					Pod: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-pod-1",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "Job",
									Name: "my-job-1",
									Controller: &trueVal,
								},
							},
						},
					},
				},
				{
					Name:      "job-pod-2",
					Namespace: "default",
					IPs:       []string{"10.0.0.2"},
					Pod: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "job-pod-2",
							Namespace: "default",
							OwnerReferences: []metav1.OwnerReference{
								{
									Kind: "Job",
									Name: "my-job-2",
									Controller: &trueVal,
								},
							},
						},
					},
				},
			},
			expected: 0, // All pods should be excluded
		},
		{
			name: "pod with no owner references",
			pods: []PodInfo{
				{
					Name:      "standalone-pod",
					Namespace: "default",
					IPs:       []string{"10.0.0.1"},
					Pod: &corev1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "standalone-pod",
							Namespace:       "default",
							OwnerReferences: []metav1.OwnerReference{},
						},
					},
				},
			},
			expected: 1, // Standalone pod should remain
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExcludeJobPods(tt.pods)
			if len(result) != tt.expected {
				t.Errorf("ExcludeJobPods() returned %d pods, expected %d", len(result), tt.expected)
			}
		})
	}
}
