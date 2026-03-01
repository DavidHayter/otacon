package diagnostics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/merthan/otacon/internal/engine"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DiagnosticEngine performs deep cluster diagnostics
type DiagnosticEngine struct {
	client kubernetes.Interface
}

// NewDiagnosticEngine creates a new diagnostic engine
func NewDiagnosticEngine(client kubernetes.Interface) *DiagnosticEngine {
	return &DiagnosticEngine{client: client}
}

// ============================================================
// NETWORK DIAGNOSTICS
// ============================================================

func (d *DiagnosticEngine) RunNetworkDiagnostics(ctx context.Context, namespace string) []engine.DiagnosticResult {
	var results []engine.DiagnosticResult
	results = append(results, d.checkCoreDNS(ctx))
	results = append(results, d.checkDNSEvents(ctx, namespace))
	results = append(results, d.checkNetworkPolicyCoverage(ctx, namespace))
	results = append(results, d.checkServicesWithoutEndpoints(ctx, namespace)...)
	results = append(results, d.checkPendingPods(ctx, namespace)...)
	return results
}

func (d *DiagnosticEngine) checkCoreDNS(ctx context.Context) engine.DiagnosticResult {
	pods, err := d.client.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=kube-dns",
	})
	if err != nil {
		return engine.DiagnosticResult{Check: "CoreDNS Status", Status: "fail",
			Message: fmt.Sprintf("Failed to check CoreDNS: %v", err)}
	}
	if pods == nil || len(pods.Items) == 0 {
		return engine.DiagnosticResult{Check: "CoreDNS Status", Status: "fail",
			Message: "No CoreDNS pods found in kube-system",
			Remediation: "Ensure CoreDNS is deployed: kubectl -n kube-system get pods -l k8s-app=kube-dns"}
	}
	running := 0
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			running++
		}
	}
	if running == len(pods.Items) {
		return engine.DiagnosticResult{Check: "CoreDNS Status", Status: "pass",
			Message: fmt.Sprintf("CoreDNS healthy: %d/%d pods running", running, len(pods.Items))}
	}
	return engine.DiagnosticResult{Check: "CoreDNS Status", Status: "warn",
		Message:     fmt.Sprintf("CoreDNS degraded: %d/%d pods running", running, len(pods.Items)),
		Remediation: "Check CoreDNS pod logs: kubectl -n kube-system logs -l k8s-app=kube-dns"}
}

func (d *DiagnosticEngine) checkDNSEvents(ctx context.Context, namespace string) engine.DiagnosticResult {
	events, err := d.client.CoreV1().Events("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return engine.DiagnosticResult{Check: "DNS Events", Status: "warn", Message: "Could not retrieve events"}
	}
	dnsIssues := 0
	var details []string
	for _, event := range events.Items {
		msg := strings.ToLower(event.Message)
		if strings.Contains(msg, "dns") || strings.Contains(msg, "resolve") ||
			strings.Contains(event.Reason, "DNSConfigForming") {
			dnsIssues++
			if len(details) < 5 {
				details = append(details, fmt.Sprintf("%s/%s: %s", event.Namespace, event.InvolvedObject.Name, event.Message))
			}
		}
	}
	if dnsIssues == 0 {
		return engine.DiagnosticResult{Check: "DNS Events", Status: "pass", Message: "No DNS-related events found"}
	}
	return engine.DiagnosticResult{Check: "DNS Events", Status: "warn",
		Message: fmt.Sprintf("Found %d DNS-related events", dnsIssues), Details: details,
		Remediation: "Check CoreDNS logs and pod DNS configuration (dnsPolicy, dnsConfig)"}
}

func (d *DiagnosticEngine) checkNetworkPolicyCoverage(ctx context.Context, namespace string) engine.DiagnosticResult {
	ns := namespace
	if ns == "" {
		ns = "default"
	}
	policies, err := d.client.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return engine.DiagnosticResult{Check: "NetworkPolicy Coverage", Status: "warn",
			Message: fmt.Sprintf("Could not check NetworkPolicies in %s", ns)}
	}
	if policies == nil || len(policies.Items) == 0 {
		return engine.DiagnosticResult{Check: "NetworkPolicy Coverage", Status: "warn",
			Message:     fmt.Sprintf("No NetworkPolicies in namespace '%s' — all traffic allowed", ns),
			Remediation: "Create at least a default-deny NetworkPolicy for the namespace"}
	}
	return engine.DiagnosticResult{Check: "NetworkPolicy Coverage", Status: "pass",
		Message: fmt.Sprintf("Found %d NetworkPolicies in namespace '%s'", len(policies.Items), ns)}
}

func (d *DiagnosticEngine) checkServicesWithoutEndpoints(ctx context.Context, namespace string) []engine.DiagnosticResult {
	var results []engine.DiagnosticResult
	services, err := d.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil || services == nil {
		return results
	}
	noEndpoints := 0
	var details []string
	for _, svc := range services.Items {
		if svc.Spec.Type == corev1.ServiceTypeExternalName || svc.Namespace == "kube-system" {
			continue
		}
		endpoints, err := d.client.CoreV1().Endpoints(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			continue
		}
		hasEndpoints := false
		for _, subset := range endpoints.Subsets {
			if len(subset.Addresses) > 0 {
				hasEndpoints = true
				break
			}
		}
		if !hasEndpoints {
			noEndpoints++
			details = append(details, fmt.Sprintf("%s/%s (type: %s)", svc.Namespace, svc.Name, svc.Spec.Type))
		}
	}
	if noEndpoints > 0 {
		results = append(results, engine.DiagnosticResult{Check: "Services Without Endpoints", Status: "warn",
			Message: fmt.Sprintf("%d services have no healthy endpoints", noEndpoints), Details: details,
			Remediation: "Check pod selector labels, pod health, and readiness probes"})
	} else {
		results = append(results, engine.DiagnosticResult{Check: "Services Without Endpoints", Status: "pass",
			Message: "All services have healthy endpoints"})
	}
	return results
}

func (d *DiagnosticEngine) checkPendingPods(ctx context.Context, namespace string) []engine.DiagnosticResult {
	var results []engine.DiagnosticResult
	pods, err := d.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{FieldSelector: "status.phase=Pending"})
	if err != nil || pods == nil || len(pods.Items) == 0 {
		results = append(results, engine.DiagnosticResult{Check: "Pending Pods", Status: "pass",
			Message: "No pods stuck in Pending state"})
		return results
	}
	var details []string
	for _, pod := range pods.Items {
		reason := "unknown"
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodScheduled && cond.Status == corev1.ConditionFalse {
				reason = cond.Message
				break
			}
		}
		details = append(details, fmt.Sprintf("%s/%s: %s", pod.Namespace, pod.Name, reason))
	}
	results = append(results, engine.DiagnosticResult{Check: "Pending Pods", Status: "fail",
		Message: fmt.Sprintf("%d pods stuck in Pending state", len(pods.Items)), Details: details,
		Remediation: "Check node resources (kubectl describe nodes) and scheduling constraints"})
	return results
}

// ============================================================
// LOG DIAGNOSTICS
// ============================================================

func (d *DiagnosticEngine) RunLogDiagnostics(ctx context.Context, namespace string) []engine.DiagnosticResult {
	var results []engine.DiagnosticResult
	results = append(results, d.checkCrashLoopPods(ctx, namespace)...)
	results = append(results, d.checkOOMKilledPods(ctx, namespace)...)
	results = append(results, d.checkErrorEvents(ctx, namespace))
	return results
}

func (d *DiagnosticEngine) checkCrashLoopPods(ctx context.Context, namespace string) []engine.DiagnosticResult {
	var results []engine.DiagnosticResult
	pods, err := d.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil || pods == nil {
		return results
	}
	crashLoops := 0
	var details []string
	for _, pod := range pods.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
				crashLoops++
				details = append(details, fmt.Sprintf("%s/%s (container: %s, restarts: %d)",
					pod.Namespace, pod.Name, cs.Name, cs.RestartCount))
				if len(details) <= 5 {
					logs := d.getLastLogs(ctx, pod.Namespace, pod.Name, cs.Name, 5)
					if logs != "" {
						details = append(details, fmt.Sprintf("  Last logs: %s", truncate(logs, 200)))
					}
				}
			}
		}
	}
	if crashLoops == 0 {
		results = append(results, engine.DiagnosticResult{Check: "CrashLoopBackOff Detection", Status: "pass",
			Message: "No pods in CrashLoopBackOff"})
	} else {
		results = append(results, engine.DiagnosticResult{Check: "CrashLoopBackOff Detection", Status: "fail",
			Message: fmt.Sprintf("%d containers in CrashLoopBackOff", crashLoops), Details: details,
			Remediation: "Check container logs: kubectl logs <pod> -c <container> --previous"})
	}
	return results
}

func (d *DiagnosticEngine) checkOOMKilledPods(ctx context.Context, namespace string) []engine.DiagnosticResult {
	var results []engine.DiagnosticResult
	pods, err := d.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil || pods == nil {
		return results
	}
	oomKills := 0
	var details []string
	for _, pod := range pods.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.LastTerminationState.Terminated != nil &&
				cs.LastTerminationState.Terminated.Reason == "OOMKilled" {
				oomKills++
				memLimit := "not set"
				for _, c := range pod.Spec.Containers {
					if c.Name == cs.Name {
						if lim := c.Resources.Limits.Memory(); lim != nil {
							memLimit = lim.String()
						}
					}
				}
				details = append(details, fmt.Sprintf("%s/%s (container: %s, memory limit: %s, restarts: %d)",
					pod.Namespace, pod.Name, cs.Name, memLimit, cs.RestartCount))
			}
		}
	}
	if oomKills == 0 {
		results = append(results, engine.DiagnosticResult{Check: "OOMKilled Detection", Status: "pass",
			Message: "No recently OOMKilled containers"})
	} else {
		results = append(results, engine.DiagnosticResult{Check: "OOMKilled Detection", Status: "fail",
			Message: fmt.Sprintf("%d containers recently OOMKilled", oomKills), Details: details,
			Remediation: "Increase memory limits or optimize application memory usage"})
	}
	return results
}

func (d *DiagnosticEngine) checkErrorEvents(ctx context.Context, namespace string) engine.DiagnosticResult {
	events, err := d.client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "type=Warning",
	})
	if err != nil || events == nil {
		return engine.DiagnosticResult{Check: "Warning Events", Status: "pass",
			Message: "Could not retrieve events"}
	}

	// Count by reason
	reasons := make(map[string]int)
	for _, event := range events.Items {
		reasons[event.Reason]++
	}

	if len(reasons) == 0 {
		return engine.DiagnosticResult{Check: "Warning Events", Status: "pass",
			Message: "No warning events found"}
	}

	var details []string
	for reason, count := range reasons {
		details = append(details, fmt.Sprintf("%s: %d occurrences", reason, count))
	}

	return engine.DiagnosticResult{
		Check:   "Warning Events Summary",
		Status:  "warn",
		Message: fmt.Sprintf("%d warning events across %d categories", len(events.Items), len(reasons)),
		Details: details,
	}
}

// ============================================================
// NODE DIAGNOSTICS
// ============================================================

func (d *DiagnosticEngine) RunNodeDiagnostics(ctx context.Context) []engine.DiagnosticResult {
	var results []engine.DiagnosticResult

	nodes, err := d.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil || nodes == nil {
		results = append(results, engine.DiagnosticResult{Check: "Node Health", Status: "fail",
			Message: fmt.Sprintf("Failed to list nodes: %v", err)})
		return results
	}

	// Check node readiness
	results = append(results, d.checkNodeReadiness(nodes.Items))

	// Check node pressure conditions
	results = append(results, d.checkNodePressure(nodes.Items)...)

	// Check node resource capacity
	results = append(results, d.checkNodeCapacity(ctx, nodes.Items))

	return results
}

func (d *DiagnosticEngine) checkNodeReadiness(nodes []corev1.Node) engine.DiagnosticResult {
	notReady := 0
	var details []string

	for _, node := range nodes {
		ready := false
		for _, cond := range node.Status.Conditions {
			if cond.Type == corev1.NodeReady {
				if cond.Status == corev1.ConditionTrue {
					ready = true
				}
				break
			}
		}
		if !ready {
			notReady++
			details = append(details, node.Name)
		}
	}

	if notReady == 0 {
		return engine.DiagnosticResult{Check: "Node Readiness", Status: "pass",
			Message: fmt.Sprintf("All %d nodes are Ready", len(nodes))}
	}

	return engine.DiagnosticResult{
		Check:       "Node Readiness",
		Status:      "fail",
		Message:     fmt.Sprintf("%d/%d nodes are NotReady", notReady, len(nodes)),
		Details:     details,
		Remediation: "Check node status: kubectl describe node <node-name>",
	}
}

func (d *DiagnosticEngine) checkNodePressure(nodes []corev1.Node) []engine.DiagnosticResult {
	var results []engine.DiagnosticResult

	pressureTypes := map[corev1.NodeConditionType]string{
		corev1.NodeMemoryPressure: "Memory Pressure",
		corev1.NodeDiskPressure:   "Disk Pressure",
		corev1.NodePIDPressure:    "PID Pressure",
	}

	for condType, name := range pressureTypes {
		affected := 0
		var details []string

		for _, node := range nodes {
			for _, cond := range node.Status.Conditions {
				if cond.Type == condType && cond.Status == corev1.ConditionTrue {
					affected++
					details = append(details, fmt.Sprintf("%s: %s", node.Name, cond.Message))
				}
			}
		}

		if affected > 0 {
			results = append(results, engine.DiagnosticResult{
				Check:       fmt.Sprintf("Node %s", name),
				Status:      "fail",
				Message:     fmt.Sprintf("%d nodes experiencing %s", affected, name),
				Details:     details,
				Remediation: fmt.Sprintf("Investigate %s on affected nodes — may need capacity expansion or workload redistribution", strings.ToLower(name)),
			})
		} else {
			results = append(results, engine.DiagnosticResult{
				Check:   fmt.Sprintf("Node %s", name),
				Status:  "pass",
				Message: fmt.Sprintf("No nodes experiencing %s", name),
			})
		}
	}

	return results
}

func (d *DiagnosticEngine) checkNodeCapacity(ctx context.Context, nodes []corev1.Node) engine.DiagnosticResult {
	totalCPU := int64(0)
	totalMem := int64(0)
	allocCPU := int64(0)
	allocMem := int64(0)

	for _, node := range nodes {
		cpu := node.Status.Allocatable.Cpu()
		mem := node.Status.Allocatable.Memory()
		if cpu != nil {
			totalCPU += cpu.MilliValue()
		}
		if mem != nil {
			totalMem += mem.Value()
		}
	}

	// Get all pods to sum requests
	pods, _ := d.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if pods != nil {
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodPending {
				continue
			}
			for _, c := range pod.Spec.Containers {
				if req := c.Resources.Requests.Cpu(); req != nil {
					allocCPU += req.MilliValue()
				}
				if req := c.Resources.Requests.Memory(); req != nil {
					allocMem += req.Value()
				}
			}
		}
	}

	cpuPct := float64(0)
	memPct := float64(0)
	if totalCPU > 0 {
		cpuPct = float64(allocCPU) / float64(totalCPU) * 100
	}
	if totalMem > 0 {
		memPct = float64(allocMem) / float64(totalMem) * 100
	}

	status := "pass"
	if cpuPct > 90 || memPct > 90 {
		status = "fail"
	} else if cpuPct > 75 || memPct > 75 {
		status = "warn"
	}

	details := []string{
		fmt.Sprintf("CPU: %.1f%% allocated (%dm / %dm)", cpuPct, allocCPU, totalCPU),
		fmt.Sprintf("Memory: %.1f%% allocated (%.1fGi / %.1fGi)", memPct,
			float64(allocMem)/(1024*1024*1024), float64(totalMem)/(1024*1024*1024)),
	}

	return engine.DiagnosticResult{
		Check:   "Cluster Resource Utilization",
		Status:  status,
		Message: fmt.Sprintf("CPU %.0f%% allocated, Memory %.0f%% allocated across %d nodes", cpuPct, memPct, len(nodes)),
		Details: details,
	}
}

// ============================================================
// HELPERS
// ============================================================

func (d *DiagnosticEngine) getLastLogs(ctx context.Context, namespace, pod, container string, lines int64) string {
	req := d.client.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{
		Container: container,
		TailLines: &lines,
		Previous:  true,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return ""
	}
	defer stream.Close()

	var buf bytes.Buffer
	io.Copy(&buf, io.LimitReader(stream, 2048))
	return strings.TrimSpace(buf.String())
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " | ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
