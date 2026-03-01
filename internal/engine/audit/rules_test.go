package audit

import (
	"context"
	"testing"

	"github.com/merthan/otacon/internal/engine"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

// ============================================================
// HELPERS
// ============================================================

func boolPtr(b bool) *bool     { return &b }
func int32Ptr(i int32) *int32  { return &i }
func int64Ptr(i int64) *int64  { return &i }

func newFakeClient(objs ...runtime.Object) *fake.Clientset {
	return fake.NewSimpleClientset(objs...)
}

func makePod(ns, name string, opts ...func(*corev1.Pod)) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "myapp:v1.0.0"},
			},
		},
	}
	for _, opt := range opts {
		opt(pod)
	}
	return pod
}

func makeDeploy(ns, name string, replicas int32, opts ...func(*appsv1.Deployment)) *appsv1.Deployment {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns, Labels: map[string]string{"app": name}},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(replicas),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: "myapp:v1.0.0"}},
				},
			},
		},
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ============================================================
// SECURITY RULE TESTS
// ============================================================

func TestCheckNoRootContainers(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		wantFind bool
	}{
		{
			name: "no security context — should flag",
			pod:  makePod("default", "test-pod"),
			wantFind: true,
		},
		{
			name: "runAsNonRoot=true — should pass",
			pod: makePod("default", "secure-pod", func(p *corev1.Pod) {
				p.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
					RunAsNonRoot: boolPtr(true),
				}
			}),
			wantFind: false,
		},
		{
			name: "runAsUser=0 — should flag",
			pod: makePod("default", "root-pod", func(p *corev1.Pod) {
				p.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
					RunAsUser: int64Ptr(0),
				}
			}),
			wantFind: true,
		},
		{
			name: "runAsUser=1000 — should pass",
			pod: makePod("default", "user-pod", func(p *corev1.Pod) {
				p.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
					RunAsUser: int64Ptr(1000),
				}
			}),
			wantFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(tt.pod)
			findings := checkNoRootContainers(context.Background(), client, "default")
			if tt.wantFind && len(findings) == 0 {
				t.Error("expected finding but got none")
			}
			if !tt.wantFind && len(findings) > 0 {
				t.Errorf("expected no findings but got %d: %v", len(findings), findings[0].Message)
			}
		})
	}
}

func TestCheckNoPrivilegedContainers(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		wantFind bool
	}{
		{
			name: "privileged=true — should flag",
			pod: makePod("default", "priv-pod", func(p *corev1.Pod) {
				p.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
					Privileged: boolPtr(true),
				}
			}),
			wantFind: true,
		},
		{
			name: "privileged=false — should pass",
			pod: makePod("default", "safe-pod", func(p *corev1.Pod) {
				p.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
					Privileged: boolPtr(false),
				}
			}),
			wantFind: false,
		},
		{
			name: "no security context — should pass (not privileged by default)",
			pod:  makePod("default", "default-pod"),
			wantFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(tt.pod)
			findings := checkNoPrivilegedContainers(context.Background(), client, "default")
			if tt.wantFind && len(findings) == 0 {
				t.Error("expected finding but got none")
			}
			if !tt.wantFind && len(findings) > 0 {
				t.Errorf("expected no findings but got %d", len(findings))
			}
		})
	}
}

func TestCheckHostNetwork(t *testing.T) {
	pod := makePod("default", "hostnet-pod", func(p *corev1.Pod) {
		p.Spec.HostNetwork = true
	})
	client := newFakeClient(pod)
	findings := checkNoHostNetwork(context.Background(), client, "default")
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

func TestCheckReadOnlyRootFS(t *testing.T) {
	securePod := makePod("default", "ro-pod", func(p *corev1.Pod) {
		p.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
			ReadOnlyRootFilesystem: boolPtr(true),
		}
	})
	client := newFakeClient(securePod)
	findings := checkReadOnlyRootFS(context.Background(), client, "default")
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for read-only FS, got %d", len(findings))
	}

	insecurePod := makePod("default", "rw-pod")
	client2 := newFakeClient(insecurePod)
	findings2 := checkReadOnlyRootFS(context.Background(), client2, "default")
	if len(findings2) == 0 {
		t.Error("expected finding for writable root FS")
	}
}

func TestCheckDropCapabilities(t *testing.T) {
	securePod := makePod("default", "cap-pod", func(p *corev1.Pod) {
		p.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		}
	})
	client := newFakeClient(securePod)
	findings := checkDropCapabilities(context.Background(), client, "default")
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// ============================================================
// RESOURCE RULE TESTS
// ============================================================

func TestCheckCPURequests(t *testing.T) {
	// Pod without CPU requests
	pod := makePod("default", "no-req-pod")
	client := newFakeClient(pod)
	findings := checkCPURequests(context.Background(), client, "default")
	if len(findings) == 0 {
		t.Error("expected finding for missing CPU requests")
	}

	// Pod with CPU requests
	podWithReq := makePod("default", "req-pod", func(p *corev1.Pod) {
		p.Spec.Containers[0].Resources.Requests = corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("100m"),
		}
	})
	client2 := newFakeClient(podWithReq)
	findings2 := checkCPURequests(context.Background(), client2, "default")
	if len(findings2) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings2))
	}
}

func TestCheckMemoryLimits(t *testing.T) {
	pod := makePod("default", "no-lim-pod")
	client := newFakeClient(pod)
	findings := checkMemoryLimits(context.Background(), client, "default")
	if len(findings) == 0 {
		t.Error("expected finding for missing memory limits")
	}
	// Should be CRITICAL severity
	if findings[0].Severity != engine.SeverityCritical {
		t.Errorf("expected CRITICAL severity, got %s", findings[0].Severity)
	}
}

// ============================================================
// RELIABILITY RULE TESTS
// ============================================================

func TestCheckLivenessProbes(t *testing.T) {
	// Pod without liveness probe
	pod := makePod("default", "no-probe-pod")
	client := newFakeClient(pod)
	findings := checkLivenessProbes(context.Background(), client, "default")
	if len(findings) == 0 {
		t.Error("expected finding for missing liveness probe")
	}

	// Pod with liveness probe
	podWithProbe := makePod("default", "probe-pod", func(p *corev1.Pod) {
		p.Spec.Containers[0].LivenessProbe = &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromInt(8080),
				},
			},
		}
	})
	client2 := newFakeClient(podWithProbe)
	findings2 := checkLivenessProbes(context.Background(), client2, "default")
	if len(findings2) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings2))
	}
}

func TestCheckSingleReplica(t *testing.T) {
	deploy := makeDeploy("default", "single-deploy", 1)
	client := newFakeClient(deploy)
	findings := checkSingleReplica(context.Background(), client, "default")
	if len(findings) == 0 {
		t.Error("expected finding for single replica")
	}

	deploy2 := makeDeploy("default", "multi-deploy", 3)
	client2 := newFakeClient(deploy2)
	findings2 := checkSingleReplica(context.Background(), client2, "default")
	if len(findings2) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings2))
	}
}

// ============================================================
// BEST PRACTICE RULE TESTS
// ============================================================

func TestCheckNoLatestTag(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		wantFind bool
	}{
		{"latest tag — should flag", "myapp:latest", true},
		{"no tag — should flag", "myapp", true},
		{"specific tag — should pass", "myapp:v1.2.3", false},
		{"digest — should pass", "myapp@sha256:abc123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := makePod("default", "img-pod", func(p *corev1.Pod) {
				p.Spec.Containers[0].Image = tt.image
			})
			client := newFakeClient(pod)
			findings := checkNoLatestTag(context.Background(), client, "default")
			if tt.wantFind && len(findings) == 0 {
				t.Error("expected finding")
			}
			if !tt.wantFind && len(findings) > 0 {
				t.Errorf("expected no findings, got %d", len(findings))
			}
		})
	}
}

// ============================================================
// SCANNER TESTS
// ============================================================

func TestScannerBuildScorecard(t *testing.T) {
	pod := makePod("default", "test-pod")
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	client := newFakeClient(pod, ns)

	scanner := NewScanner(client)
	scorecard, err := scanner.Scan(context.Background(), ScanOptions{
		Namespace: "default",
		Workers:   2,
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if scorecard.Grade == "" {
		t.Error("grade should not be empty")
	}
	if len(scorecard.Categories) != 5 {
		t.Errorf("expected 5 categories, got %d", len(scorecard.Categories))
	}
	if scorecard.OverallScore < 0 || scorecard.OverallScore > 100 {
		t.Errorf("overall score out of range: %f", scorecard.OverallScore)
	}
}

func TestGetTopFindings(t *testing.T) {
	scorecard := &engine.Scorecard{
		Categories: []engine.CategoryScore{
			{
				Findings: []engine.Finding{
					{Severity: engine.SeverityInfo, Message: "info1"},
					{Severity: engine.SeverityCritical, Message: "crit1"},
					{Severity: engine.SeverityWarning, Message: "warn1"},
				},
			},
		},
	}

	top := GetTopFindings(scorecard, 2)
	if len(top) != 2 {
		t.Errorf("expected 2 top findings, got %d", len(top))
	}
	// First should be critical
	if top[0].Severity != engine.SeverityCritical {
		t.Errorf("expected critical first, got %s", top[0].Severity)
	}
}

// Suppress unused import warnings
var _ = intstr.FromInt
