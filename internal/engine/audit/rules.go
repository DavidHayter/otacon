package audit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/merthan/otacon/internal/engine"
	corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Rule represents a single audit check
type Rule struct {
	ID          string
	Name        string
	Category    string
	Severity    engine.Severity
	Description string
	Explain     string
	Check       func(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding
}

// BuiltinRules returns all built-in audit rules
func BuiltinRules() []Rule {
	var rules []Rule
	rules = append(rules, securityRules()...)
	rules = append(rules, resourceRules()...)
	rules = append(rules, reliabilityRules()...)
	rules = append(rules, bestPracticeRules()...)
	rules = append(rules, networkRules()...)
	return rules
}

// ============================================================
// SECURITY RULES
// ============================================================

func securityRules() []Rule {
	return []Rule{
		{
			ID:       "SEC-001",
			Name:     "no-root-containers",
			Category: "Security",
			Severity: engine.SeverityCritical,
			Description: "Containers must not run as root",
			Explain: "A container running as root (UID 0) poses a significant security risk. " +
				"If an attacker exploits a container escape vulnerability, they gain root access " +
				"to the host node, potentially compromising the entire cluster.",
			Check: checkNoRootContainers,
		},
		{
			ID:       "SEC-002",
			Name:     "no-privileged-containers",
			Category: "Security",
			Severity: engine.SeverityCritical,
			Description: "Containers must not run in privileged mode",
			Explain: "Privileged containers have full access to host devices and kernel capabilities. " +
				"This effectively removes all container isolation, making the container equivalent " +
				"to a process running directly on the host.",
			Check: checkNoPrivilegedContainers,
		},
		{
			ID:       "SEC-003",
			Name:     "no-host-network",
			Category: "Security",
			Severity: engine.SeverityWarning,
			Description: "Pods should not use host networking",
			Explain: "Using host networking bypasses network isolation, allowing the pod to access " +
				"all network interfaces on the host. This can expose sensitive services and " +
				"circumvent NetworkPolicy enforcement.",
			Check: checkNoHostNetwork,
		},
		{
			ID:       "SEC-004",
			Name:     "no-host-pid",
			Category: "Security",
			Severity: engine.SeverityWarning,
			Description: "Pods should not share host PID namespace",
			Explain: "Sharing the host PID namespace allows processes in the pod to see and " +
				"potentially interact with all processes on the host node.",
			Check: checkNoHostPID,
		},
		{
			ID:       "SEC-005",
			Name:     "no-host-ipc",
			Category: "Security",
			Severity: engine.SeverityWarning,
			Description: "Pods should not share host IPC namespace",
			Explain: "Sharing the host IPC namespace allows the pod to communicate with " +
				"host-level processes via shared memory segments.",
			Check: checkNoHostIPC,
		},
		{
			ID:       "SEC-006",
			Name:     "read-only-root-filesystem",
			Category: "Security",
			Severity: engine.SeverityWarning,
			Description: "Containers should use read-only root filesystem",
			Explain: "A read-only root filesystem prevents attackers from writing malicious " +
				"executables or modifying system files inside the container. Use emptyDir " +
				"or tmpfs volumes for writable paths.",
			Check: checkReadOnlyRootFS,
		},
		{
			ID:       "SEC-007",
			Name:     "no-capability-escalation",
			Category: "Security",
			Severity: engine.SeverityCritical,
			Description: "Containers should not allow privilege escalation",
			Explain: "AllowPrivilegeEscalation enables a process inside the container to gain " +
				"more privileges than its parent process. This should be explicitly set to false.",
			Check: checkNoPrivilegeEscalation,
		},
		{
			ID:       "SEC-008",
			Name:     "drop-all-capabilities",
			Category: "Security",
			Severity: engine.SeverityWarning,
			Description: "Containers should drop all capabilities and add only required ones",
			Explain: "Linux capabilities provide fine-grained access control. Containers should " +
				"drop ALL capabilities and selectively add only the ones needed. Dangerous " +
				"capabilities include SYS_ADMIN, NET_RAW, and SYS_PTRACE.",
			Check: checkDropCapabilities,
		},
		{
			ID:       "SEC-009",
			Name:     "no-default-service-account",
			Category: "Security",
			Severity: engine.SeverityInfo,
			Description: "Pods should not use the default service account",
			Explain: "The default service account may have more permissions than needed. " +
				"Create dedicated service accounts with minimal RBAC permissions for each workload.",
			Check: checkNoDefaultServiceAccount,
		},
		{
			ID:       "SEC-010",
			Name:     "no-automount-service-token",
			Category: "Security",
			Severity: engine.SeverityWarning,
			Description: "Service account token should not be auto-mounted unless needed",
			Explain: "Auto-mounting the service account token makes it available to all containers " +
				"in the pod. If the workload doesn't need Kubernetes API access, disable auto-mounting " +
				"to reduce the attack surface.",
			Check: checkNoAutoMountToken,
		},
		{
			ID:       "SEC-011",
			Name:     "image-pull-policy-always",
			Category: "Security",
			Severity: engine.SeverityInfo,
			Description: "Production images should use 'Always' pull policy",
			Explain: "Using 'Always' image pull policy ensures you're always running the " +
				"intended version and prevents running stale cached images that may have " +
				"known vulnerabilities.",
			Check: checkImagePullPolicy,
		},
	}
}

// ============================================================
// RESOURCE MANAGEMENT RULES
// ============================================================

func resourceRules() []Rule {
	return []Rule{
		{
			ID:       "RES-001",
			Name:     "cpu-requests-defined",
			Category: "Resource Management",
			Severity: engine.SeverityWarning,
			Description: "Containers must have CPU requests defined",
			Explain: "Without CPU requests, the scheduler cannot make informed decisions about " +
				"pod placement. This leads to resource contention and unpredictable performance. " +
				"CPU requests guarantee a minimum amount of CPU time for your container.",
			Check: checkCPURequests,
		},
		{
			ID:       "RES-002",
			Name:     "memory-requests-defined",
			Category: "Resource Management",
			Severity: engine.SeverityWarning,
			Description: "Containers must have memory requests defined",
			Explain: "Without memory requests, pods can be scheduled on nodes without sufficient " +
				"memory, leading to OOMKills and evictions. Memory requests ensure the pod " +
				"gets the memory it needs to function.",
			Check: checkMemoryRequests,
		},
		{
			ID:       "RES-003",
			Name:     "cpu-limits-defined",
			Category: "Resource Management",
			Severity: engine.SeverityWarning,
			Description: "Containers must have CPU limits defined",
			Explain: "Without CPU limits, a single container can consume all CPU on a node, " +
				"starving other workloads. Set CPU limits to prevent noisy neighbor issues.",
			Check: checkCPULimits,
		},
		{
			ID:       "RES-004",
			Name:     "memory-limits-defined",
			Category: "Resource Management",
			Severity: engine.SeverityCritical,
			Description: "Containers must have memory limits defined",
			Explain: "Without memory limits, a memory leak can consume all node memory, " +
				"triggering the OOM killer and potentially crashing critical system pods. " +
				"Memory limits are the most important resource constraint to set.",
			Check: checkMemoryLimits,
		},
		{
			ID:       "RES-005",
			Name:     "resource-ratio-check",
			Category: "Resource Management",
			Severity: engine.SeverityInfo,
			Description: "CPU/Memory limit-to-request ratio should be reasonable",
			Explain: "A very high limit-to-request ratio (>5x) indicates potential resource " +
				"overcommitment. While some overcommit is normal, extreme ratios suggest " +
				"the requests or limits need adjustment.",
			Check: checkResourceRatios,
		},
		{
			ID:       "RES-006",
			Name:     "namespace-resource-quota",
			Category: "Resource Management",
			Severity: engine.SeverityWarning,
			Description: "Namespaces should have ResourceQuotas defined",
			Explain: "ResourceQuotas prevent a single namespace from consuming all cluster " +
				"resources. Without quotas, a misconfigured deployment could exhaust node " +
				"capacity and impact other teams.",
			Check: checkResourceQuotas,
		},
		{
			ID:       "RES-007",
			Name:     "namespace-limit-range",
			Category: "Resource Management",
			Severity: engine.SeverityInfo,
			Description: "Namespaces should have LimitRanges for default constraints",
			Explain: "LimitRanges provide default resource requests and limits for containers " +
				"that don't specify them. This acts as a safety net against workloads deployed " +
				"without resource specifications.",
			Check: checkLimitRanges,
		},
		{
			ID:       "RES-008",
			Name:     "hpa-configuration",
			Category: "Resource Management",
			Severity: engine.SeverityInfo,
			Description: "Deployments with >1 replica should consider HPA",
			Explain: "Horizontal Pod Autoscaler automatically adjusts the number of replicas " +
				"based on observed metrics. For workloads with variable load, HPA ensures " +
				"efficient resource utilization and availability.",
			Check: checkHPAConfiguration,
		},
	}
}

// ============================================================
// RELIABILITY RULES
// ============================================================

func reliabilityRules() []Rule {
	return []Rule{
		{
			ID:       "REL-001",
			Name:     "liveness-probe-defined",
			Category: "Reliability",
			Severity: engine.SeverityWarning,
			Description: "Containers should have liveness probes defined",
			Explain: "Liveness probes tell Kubernetes when to restart a container. Without them, " +
				"a container in a deadlock or hung state will continue running but not serving " +
				"traffic, requiring manual intervention.",
			Check: checkLivenessProbes,
		},
		{
			ID:       "REL-002",
			Name:     "readiness-probe-defined",
			Category: "Reliability",
			Severity: engine.SeverityWarning,
			Description: "Containers should have readiness probes defined",
			Explain: "Readiness probes determine when a container is ready to receive traffic. " +
				"Without them, traffic is sent to pods immediately after start, potentially " +
				"causing errors during initialization.",
			Check: checkReadinessProbes,
		},
		{
			ID:       "REL-003",
			Name:     "startup-probe-for-slow-start",
			Category: "Reliability",
			Severity: engine.SeverityInfo,
			Description: "Slow-starting containers should have startup probes",
			Explain: "Startup probes prevent liveness probes from killing containers that take " +
				"a long time to initialize (e.g., Java applications). Without startup probes, " +
				"you need to set a very high initialDelaySeconds on the liveness probe.",
			Check: checkStartupProbes,
		},
		{
			ID:       "REL-004",
			Name:     "single-replica-detection",
			Category: "Reliability",
			Severity: engine.SeverityWarning,
			Description: "Production deployments should have more than 1 replica",
			Explain: "Running a single replica means any pod disruption (node failure, deployment, " +
				"OOM) causes complete service downtime. Production workloads should run at least " +
				"2 replicas for basic availability.",
			Check: checkSingleReplica,
		},
		{
			ID:       "REL-005",
			Name:     "pod-disruption-budget",
			Category: "Reliability",
			Severity: engine.SeverityWarning,
			Description: "Deployments with >1 replica should have PodDisruptionBudgets",
			Explain: "PodDisruptionBudgets protect against voluntary disruptions (node drain, " +
				"cluster upgrades) by ensuring a minimum number of pods remain available. " +
				"Without PDBs, a node drain could take down all replicas simultaneously.",
			Check: checkPodDisruptionBudgets,
		},
		{
			ID:       "REL-006",
			Name:     "pod-anti-affinity",
			Category: "Reliability",
			Severity: engine.SeverityInfo,
			Description: "Multi-replica deployments should spread across nodes",
			Explain: "Without pod anti-affinity rules, all replicas of a deployment may be " +
				"scheduled on the same node. If that node fails, all replicas are lost. " +
				"Anti-affinity ensures replicas are distributed across failure domains.",
			Check: checkAntiAffinity,
		},
		{
			ID:       "REL-007",
			Name:     "restart-policy-check",
			Category: "Reliability",
			Severity: engine.SeverityInfo,
			Description: "Pod restart policy should match workload type",
			Explain: "Long-running services should use 'Always' restart policy, while Jobs " +
				"should use 'OnFailure' or 'Never'. Incorrect restart policies can mask " +
				"issues or cause unnecessary restarts.",
			Check: checkRestartPolicy,
		},
	}
}

// ============================================================
// BEST PRACTICES RULES
// ============================================================

func bestPracticeRules() []Rule {
	return []Rule{
		{
			ID:       "BP-001",
			Name:     "no-latest-tag",
			Category: "Best Practices",
			Severity: engine.SeverityWarning,
			Description: "Container images should not use 'latest' tag",
			Explain: "The 'latest' tag is mutable — it points to a different image after each " +
				"push. This makes deployments non-reproducible, complicates rollbacks, and " +
				"can introduce unexpected changes. Always use specific version tags or digests.",
			Check: checkNoLatestTag,
		},
		{
			ID:       "BP-002",
			Name:     "required-labels",
			Category: "Best Practices",
			Severity: engine.SeverityInfo,
			Description: "Workloads should have standard labels (app, team, version, environment)",
			Explain: "Standard labels enable consistent resource identification, monitoring, " +
				"cost allocation, and operational tooling across the cluster. Missing labels " +
				"make it harder to track ownership and manage resources.",
			Check: checkRequiredLabels,
		},
		{
			ID:       "BP-003",
			Name:     "container-image-registry",
			Category: "Best Practices",
			Severity: engine.SeverityInfo,
			Description: "Container images should come from approved registries",
			Explain: "Using images from unapproved registries increases the risk of running " +
				"malicious or vulnerable containers. Establish a list of trusted registries " +
				"and enforce their use.",
			Check: checkImageRegistries,
		},
		{
			ID:       "BP-004",
			Name:     "deployment-strategy",
			Category: "Best Practices",
			Severity: engine.SeverityInfo,
			Description: "Deployments should define an update strategy",
			Explain: "The default RollingUpdate strategy is generally good, but you should " +
				"configure maxSurge and maxUnavailable to match your application's tolerance " +
				"for disruption during updates.",
			Check: checkDeploymentStrategy,
		},
		{
			ID:       "BP-005",
			Name:     "termination-grace-period",
			Category: "Best Practices",
			Severity: engine.SeverityInfo,
			Description: "Pods should have appropriate terminationGracePeriodSeconds",
			Explain: "The default 30-second grace period may not be enough for applications " +
				"that need to drain connections, flush buffers, or complete in-flight requests. " +
				"Set this based on your application's shutdown requirements.",
			Check: checkTerminationGracePeriod,
		},
	}
}

// ============================================================
// NETWORK RULES
// ============================================================

func networkRules() []Rule {
	return []Rule{
		{
			ID:       "NET-001",
			Name:     "network-policy-exists",
			Category: "Network Policies",
			Severity: engine.SeverityCritical,
			Description: "Namespaces should have NetworkPolicies defined",
			Explain: "Without NetworkPolicies, all pods can communicate with each other freely. " +
				"This violates the principle of least privilege and means a compromised pod can " +
				"access any service in the cluster. Define ingress and egress policies.",
			Check: checkNetworkPolicies,
		},
		{
			ID:       "NET-002",
			Name:     "default-deny-policy",
			Category: "Network Policies",
			Severity: engine.SeverityWarning,
			Description: "Namespaces should have a default-deny NetworkPolicy",
			Explain: "A default-deny policy blocks all traffic by default, requiring explicit " +
				"allow rules. This ensures new workloads are secure by default and only " +
				"authorized communication paths are open.",
			Check: checkDefaultDenyPolicy,
		},
		{
			ID:       "NET-003",
			Name:     "service-type-check",
			Category: "Network Policies",
			Severity: engine.SeverityWarning,
			Description: "Services should not use NodePort or LoadBalancer unnecessarily",
			Explain: "NodePort and LoadBalancer services expose pods directly to external traffic. " +
				"Prefer ClusterIP services behind an Ingress controller for better security " +
				"and traffic management.",
			Check: checkServiceTypes,
		},
	}
}

// ============================================================
// CHECK IMPLEMENTATIONS
// ============================================================

func checkNoRootContainers(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			isRoot := false
			if container.SecurityContext == nil {
				isRoot = true
			} else if container.SecurityContext.RunAsNonRoot == nil || !*container.SecurityContext.RunAsNonRoot {
				if container.SecurityContext.RunAsUser == nil || *container.SecurityContext.RunAsUser == 0 {
					isRoot = true
				}
			}

			if isRoot {
				findings = append(findings, engine.Finding{
					ID:        "SEC-001",
					Category:  "Security",
					Rule:      "no-root-containers",
					Severity:  engine.SeverityCritical,
					Resource:  fmt.Sprintf("%s/%s (container: %s)", pod.Namespace, pod.Name, container.Name),
					Namespace: pod.Namespace,
					Kind:      "Pod",
					Message:   fmt.Sprintf("Container '%s' may run as root (no runAsNonRoot or runAsUser set)", container.Name),
					Remediation: "spec.containers[*].securityContext:\n  runAsNonRoot: true\n  runAsUser: 1000",
					Timestamp: time.Now(),
				})
			}
		}
	}
	return findings
}

func checkNoPrivilegedContainers(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if container.SecurityContext != nil && container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
				findings = append(findings, engine.Finding{
					ID:        "SEC-002",
					Category:  "Security",
					Rule:      "no-privileged-containers",
					Severity:  engine.SeverityCritical,
					Resource:  fmt.Sprintf("%s/%s (container: %s)", pod.Namespace, pod.Name, container.Name),
					Namespace: pod.Namespace,
					Kind:      "Pod",
					Message:   fmt.Sprintf("Container '%s' runs in privileged mode", container.Name),
					Remediation: "spec.containers[*].securityContext.privileged: false",
					Timestamp: time.Now(),
				})
			}
		}
	}
	return findings
}

func checkNoHostNetwork(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		if pod.Spec.HostNetwork {
			findings = append(findings, engine.Finding{
				ID:        "SEC-003",
				Category:  "Security",
				Rule:      "no-host-network",
				Severity:  engine.SeverityWarning,
				Resource:  fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
				Namespace: pod.Namespace,
				Kind:      "Pod",
				Message:   "Pod uses host networking",
				Remediation: "spec.hostNetwork: false",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

func checkNoHostPID(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		if pod.Spec.HostPID {
			findings = append(findings, engine.Finding{
				ID:        "SEC-004",
				Category:  "Security",
				Rule:      "no-host-pid",
				Severity:  engine.SeverityWarning,
				Resource:  fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
				Namespace: pod.Namespace,
				Kind:      "Pod",
				Message:   "Pod shares host PID namespace",
				Remediation: "spec.hostPID: false",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

func checkNoHostIPC(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		if pod.Spec.HostIPC {
			findings = append(findings, engine.Finding{
				ID:        "SEC-005",
				Category:  "Security",
				Rule:      "no-host-ipc",
				Severity:  engine.SeverityWarning,
				Resource:  fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
				Namespace: pod.Namespace,
				Kind:      "Pod",
				Message:   "Pod shares host IPC namespace",
				Remediation: "spec.hostIPC: false",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

func checkReadOnlyRootFS(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if container.SecurityContext == nil || container.SecurityContext.ReadOnlyRootFilesystem == nil || !*container.SecurityContext.ReadOnlyRootFilesystem {
				findings = append(findings, engine.Finding{
					ID:        "SEC-006",
					Category:  "Security",
					Rule:      "read-only-root-filesystem",
					Severity:  engine.SeverityWarning,
					Resource:  fmt.Sprintf("%s/%s (container: %s)", pod.Namespace, pod.Name, container.Name),
					Namespace: pod.Namespace,
					Kind:      "Pod",
					Message:   fmt.Sprintf("Container '%s' does not use read-only root filesystem", container.Name),
					Remediation: "spec.containers[*].securityContext.readOnlyRootFilesystem: true",
					Timestamp: time.Now(),
				})
			}
		}
	}
	return findings
}

func checkNoPrivilegeEscalation(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if container.SecurityContext == nil || container.SecurityContext.AllowPrivilegeEscalation == nil || *container.SecurityContext.AllowPrivilegeEscalation {
				findings = append(findings, engine.Finding{
					ID:        "SEC-007",
					Category:  "Security",
					Rule:      "no-capability-escalation",
					Severity:  engine.SeverityCritical,
					Resource:  fmt.Sprintf("%s/%s (container: %s)", pod.Namespace, pod.Name, container.Name),
					Namespace: pod.Namespace,
					Kind:      "Pod",
					Message:   fmt.Sprintf("Container '%s' allows privilege escalation", container.Name),
					Remediation: "spec.containers[*].securityContext.allowPrivilegeEscalation: false",
					Timestamp: time.Now(),
				})
			}
		}
	}
	return findings
}

func checkDropCapabilities(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			dropsAll := false
			if container.SecurityContext != nil && container.SecurityContext.Capabilities != nil {
				for _, cap := range container.SecurityContext.Capabilities.Drop {
					if cap == "ALL" {
						dropsAll = true
						break
					}
				}
			}

			if !dropsAll {
				findings = append(findings, engine.Finding{
					ID:        "SEC-008",
					Category:  "Security",
					Rule:      "drop-all-capabilities",
					Severity:  engine.SeverityWarning,
					Resource:  fmt.Sprintf("%s/%s (container: %s)", pod.Namespace, pod.Name, container.Name),
					Namespace: pod.Namespace,
					Kind:      "Pod",
					Message:   fmt.Sprintf("Container '%s' does not drop ALL capabilities", container.Name),
					Remediation: "spec.containers[*].securityContext.capabilities:\n  drop: [\"ALL\"]\n  add: [\"NET_BIND_SERVICE\"]  # only if needed",
					Timestamp: time.Now(),
				})
			}
		}
	}
	return findings
}

func checkNoDefaultServiceAccount(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		if pod.Spec.ServiceAccountName == "" || pod.Spec.ServiceAccountName == "default" {
			findings = append(findings, engine.Finding{
				ID:        "SEC-009",
				Category:  "Security",
				Rule:      "no-default-service-account",
				Severity:  engine.SeverityInfo,
				Resource:  fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
				Namespace: pod.Namespace,
				Kind:      "Pod",
				Message:   "Pod uses the default service account",
				Remediation: "spec.serviceAccountName: <dedicated-sa>",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

func checkNoAutoMountToken(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
			findings = append(findings, engine.Finding{
				ID:        "SEC-010",
				Category:  "Security",
				Rule:      "no-automount-service-token",
				Severity:  engine.SeverityWarning,
				Resource:  fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
				Namespace: pod.Namespace,
				Kind:      "Pod",
				Message:   "Service account token is auto-mounted",
				Remediation: "spec.automountServiceAccountToken: false",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

func checkImagePullPolicy(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if container.ImagePullPolicy == corev1.PullIfNotPresent || container.ImagePullPolicy == corev1.PullNever {
				// Only flag if not using a digest
				if !strings.Contains(container.Image, "@sha256:") {
					findings = append(findings, engine.Finding{
						ID:        "SEC-011",
						Category:  "Security",
						Rule:      "image-pull-policy-always",
						Severity:  engine.SeverityInfo,
						Resource:  fmt.Sprintf("%s/%s (container: %s)", pod.Namespace, pod.Name, container.Name),
						Namespace: pod.Namespace,
						Kind:      "Pod",
						Message:   fmt.Sprintf("Container '%s' uses '%s' pull policy without image digest", container.Name, container.ImagePullPolicy),
						Remediation: "spec.containers[*].imagePullPolicy: Always\n# Or use image digest: image@sha256:abc123...",
						Timestamp: time.Now(),
					})
				}
			}
		}
	}
	return findings
}

// Resource checks
func checkCPURequests(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	return checkResourceField(ctx, client, namespace, "RES-001", "cpu-requests-defined", "CPU request", func(c corev1.Container) bool {
		return c.Resources.Requests.Cpu() != nil && !c.Resources.Requests.Cpu().IsZero()
	})
}

func checkMemoryRequests(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	return checkResourceField(ctx, client, namespace, "RES-002", "memory-requests-defined", "memory request", func(c corev1.Container) bool {
		return c.Resources.Requests.Memory() != nil && !c.Resources.Requests.Memory().IsZero()
	})
}

func checkCPULimits(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	return checkResourceField(ctx, client, namespace, "RES-003", "cpu-limits-defined", "CPU limit", func(c corev1.Container) bool {
		return c.Resources.Limits.Cpu() != nil && !c.Resources.Limits.Cpu().IsZero()
	})
}

func checkMemoryLimits(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	return checkResourceField(ctx, client, namespace, "RES-004", "memory-limits-defined", "memory limit", func(c corev1.Container) bool {
		return c.Resources.Limits.Memory() != nil && !c.Resources.Limits.Memory().IsZero()
	})
}

func checkResourceField(ctx context.Context, client kubernetes.Interface, namespace, id, rule, field string, check func(corev1.Container) bool) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	severity := engine.SeverityWarning
	if id == "RES-004" {
		severity = engine.SeverityCritical
	}

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if !check(container) {
				findings = append(findings, engine.Finding{
					ID:        id,
					Category:  "Resource Management",
					Rule:      rule,
					Severity:  severity,
					Resource:  fmt.Sprintf("%s/%s (container: %s)", pod.Namespace, pod.Name, container.Name),
					Namespace: pod.Namespace,
					Kind:      "Pod",
					Message:   fmt.Sprintf("Container '%s' has no %s defined", container.Name, field),
					Remediation: fmt.Sprintf("spec.containers[*].resources.requests/limits — set appropriate %s", field),
					Timestamp: time.Now(),
				})
			}
		}
	}
	return findings
}

func checkResourceRatios(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			cpuReq := container.Resources.Requests.Cpu()
			cpuLim := container.Resources.Limits.Cpu()
			memReq := container.Resources.Requests.Memory()
			memLim := container.Resources.Limits.Memory()

			if cpuReq != nil && cpuLim != nil && !cpuReq.IsZero() {
				ratio := float64(cpuLim.MilliValue()) / float64(cpuReq.MilliValue())
				if ratio > 5.0 {
					findings = append(findings, engine.Finding{
						ID:        "RES-005",
						Category:  "Resource Management",
						Rule:      "resource-ratio-check",
						Severity:  engine.SeverityInfo,
						Resource:  fmt.Sprintf("%s/%s (container: %s)", pod.Namespace, pod.Name, container.Name),
						Namespace: pod.Namespace,
						Kind:      "Pod",
						Message:   fmt.Sprintf("Container '%s' CPU limit/request ratio is %.1fx (limit: %s, request: %s)", container.Name, ratio, cpuLim.String(), cpuReq.String()),
						Timestamp: time.Now(),
					})
				}
			}

			if memReq != nil && memLim != nil && !memReq.IsZero() {
				ratio := float64(memLim.Value()) / float64(memReq.Value())
				if ratio > 5.0 {
					findings = append(findings, engine.Finding{
						ID:        "RES-005",
						Category:  "Resource Management",
						Rule:      "resource-ratio-check",
						Severity:  engine.SeverityInfo,
						Resource:  fmt.Sprintf("%s/%s (container: %s)", pod.Namespace, pod.Name, container.Name),
						Namespace: pod.Namespace,
						Kind:      "Pod",
						Message:   fmt.Sprintf("Container '%s' Memory limit/request ratio is %.1fx", container.Name, ratio),
						Timestamp: time.Now(),
					})
				}
			}
		}
	}
	return findings
}

func checkResourceQuotas(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	namespaces := listNamespaces(ctx, client, namespace)

	for _, ns := range namespaces {
		quotas, _ := client.CoreV1().ResourceQuotas(ns.Name).List(ctx, metav1.ListOptions{})
		if quotas == nil || len(quotas.Items) == 0 {
			findings = append(findings, engine.Finding{
				ID:        "RES-006",
				Category:  "Resource Management",
				Rule:      "namespace-resource-quota",
				Severity:  engine.SeverityWarning,
				Resource:  ns.Name,
				Namespace: ns.Name,
				Kind:      "Namespace",
				Message:   fmt.Sprintf("Namespace '%s' has no ResourceQuota defined", ns.Name),
				Remediation: "Create a ResourceQuota to limit total resource consumption in this namespace",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

func checkLimitRanges(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	namespaces := listNamespaces(ctx, client, namespace)

	for _, ns := range namespaces {
		ranges, _ := client.CoreV1().LimitRanges(ns.Name).List(ctx, metav1.ListOptions{})
		if ranges == nil || len(ranges.Items) == 0 {
			findings = append(findings, engine.Finding{
				ID:        "RES-007",
				Category:  "Resource Management",
				Rule:      "namespace-limit-range",
				Severity:  engine.SeverityInfo,
				Resource:  ns.Name,
				Namespace: ns.Name,
				Kind:      "Namespace",
				Message:   fmt.Sprintf("Namespace '%s' has no LimitRange defined", ns.Name),
				Remediation: "Create a LimitRange to set default requests/limits for containers",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

func checkHPAConfiguration(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	// HPA check is informational — skipped for now, requires autoscaling API
	return nil
}

// Reliability checks
func checkLivenessProbes(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	return checkProbe(ctx, client, namespace, "REL-001", "liveness-probe-defined", "liveness", func(c corev1.Container) bool {
		return c.LivenessProbe != nil
	})
}

func checkReadinessProbes(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	return checkProbe(ctx, client, namespace, "REL-002", "readiness-probe-defined", "readiness", func(c corev1.Container) bool {
		return c.ReadinessProbe != nil
	})
}

func checkStartupProbes(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	// Only flag containers with liveness but no startup and high initial delay
	return nil
}

func checkProbe(ctx context.Context, client kubernetes.Interface, namespace, id, rule, probeType string, check func(corev1.Container) bool) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if !check(container) {
				findings = append(findings, engine.Finding{
					ID:        id,
					Category:  "Reliability",
					Rule:      rule,
					Severity:  engine.SeverityWarning,
					Resource:  fmt.Sprintf("%s/%s (container: %s)", pod.Namespace, pod.Name, container.Name),
					Namespace: pod.Namespace,
					Kind:      "Pod",
					Message:   fmt.Sprintf("Container '%s' has no %s probe defined", container.Name, probeType),
					Remediation: fmt.Sprintf("spec.containers[*].%sProbe: { httpGet: { path: /healthz, port: 8080 } }", probeType),
					Timestamp: time.Now(),
				})
			}
		}
	}
	return findings
}

func checkSingleReplica(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	deployments := listDeployments(ctx, client, namespace)

	for _, deploy := range deployments {
		if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == 1 {
			findings = append(findings, engine.Finding{
				ID:        "REL-004",
				Category:  "Reliability",
				Rule:      "single-replica-detection",
				Severity:  engine.SeverityWarning,
				Resource:  fmt.Sprintf("%s/%s", deploy.Namespace, deploy.Name),
				Namespace: deploy.Namespace,
				Kind:      "Deployment",
				Message:   fmt.Sprintf("Deployment '%s' runs with only 1 replica", deploy.Name),
				Remediation: "spec.replicas: 2  # minimum for availability",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

func checkPodDisruptionBudgets(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	deployments := listDeployments(ctx, client, namespace)

	for _, deploy := range deployments {
		if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas > 1 {
			pdbs, _ := client.PolicyV1().PodDisruptionBudgets(deploy.Namespace).List(ctx, metav1.ListOptions{})
			hasPDB := false
			if pdbs != nil {
				for _, pdb := range pdbs.Items {
					if pdb.Spec.Selector != nil {
						// Simple check: see if any PDB selector matches deployment labels
						match := true
						for k, v := range pdb.Spec.Selector.MatchLabels {
							if deploy.Spec.Template.Labels[k] != v {
								match = false
								break
							}
						}
						if match {
							hasPDB = true
							break
						}
					}
				}
			}
			if !hasPDB {
				findings = append(findings, engine.Finding{
					ID:        "REL-005",
					Category:  "Reliability",
					Rule:      "pod-disruption-budget",
					Severity:  engine.SeverityWarning,
					Resource:  fmt.Sprintf("%s/%s", deploy.Namespace, deploy.Name),
					Namespace: deploy.Namespace,
					Kind:      "Deployment",
					Message:   fmt.Sprintf("Deployment '%s' has %d replicas but no PodDisruptionBudget", deploy.Name, *deploy.Spec.Replicas),
					Remediation: "Create a PodDisruptionBudget with minAvailable or maxUnavailable",
					Timestamp: time.Now(),
				})
			}
		}
	}
	return findings
}

func checkAntiAffinity(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	deployments := listDeployments(ctx, client, namespace)

	for _, deploy := range deployments {
		if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas > 1 {
			affinity := deploy.Spec.Template.Spec.Affinity
			if affinity == nil || affinity.PodAntiAffinity == nil {
				findings = append(findings, engine.Finding{
					ID:        "REL-006",
					Category:  "Reliability",
					Rule:      "pod-anti-affinity",
					Severity:  engine.SeverityInfo,
					Resource:  fmt.Sprintf("%s/%s", deploy.Namespace, deploy.Name),
					Namespace: deploy.Namespace,
					Kind:      "Deployment",
					Message:   fmt.Sprintf("Deployment '%s' has %d replicas but no pod anti-affinity", deploy.Name, *deploy.Spec.Replicas),
					Remediation: "spec.template.spec.affinity.podAntiAffinity with topologyKey: kubernetes.io/hostname",
					Timestamp: time.Now(),
				})
			}
		}
	}
	return findings
}

func checkRestartPolicy(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	return nil // Informational, skip for v1
}

// Best practice checks
func checkNoLatestTag(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	pods := listPods(ctx, client, namespace)

	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			image := container.Image
			if strings.HasSuffix(image, ":latest") || (!strings.Contains(image, ":") && !strings.Contains(image, "@")) {
				findings = append(findings, engine.Finding{
					ID:        "BP-001",
					Category:  "Best Practices",
					Rule:      "no-latest-tag",
					Severity:  engine.SeverityWarning,
					Resource:  fmt.Sprintf("%s/%s (container: %s)", pod.Namespace, pod.Name, container.Name),
					Namespace: pod.Namespace,
					Kind:      "Pod",
					Message:   fmt.Sprintf("Container '%s' uses 'latest' or untagged image: %s", container.Name, image),
					Remediation: "Use a specific version tag or image digest: myapp:v1.2.3 or myapp@sha256:...",
					Timestamp: time.Now(),
				})
			}
		}
	}
	return findings
}

func checkRequiredLabels(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	deployments := listDeployments(ctx, client, namespace)
	required := []string{"app", "team", "version", "environment"}

	for _, deploy := range deployments {
		var missing []string
		for _, label := range required {
			if _, ok := deploy.Labels[label]; !ok {
				// Also check template labels
				if _, ok := deploy.Spec.Template.Labels[label]; !ok {
					missing = append(missing, label)
				}
			}
		}
		if len(missing) > 0 {
			findings = append(findings, engine.Finding{
				ID:        "BP-002",
				Category:  "Best Practices",
				Rule:      "required-labels",
				Severity:  engine.SeverityInfo,
				Resource:  fmt.Sprintf("%s/%s", deploy.Namespace, deploy.Name),
				Namespace: deploy.Namespace,
				Kind:      "Deployment",
				Message:   fmt.Sprintf("Deployment '%s' missing labels: %s", deploy.Name, strings.Join(missing, ", ")),
				Remediation: "Add standard labels: app, team, version, environment",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

func checkImageRegistries(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	return nil // Requires user-defined approved registries, skip for v1
}

func checkDeploymentStrategy(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	return nil // Informational
}

func checkTerminationGracePeriod(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	return nil // Informational
}

// Network checks
func checkNetworkPolicies(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	namespaces := listNamespaces(ctx, client, namespace)

	for _, ns := range namespaces {
		policies, _ := client.NetworkingV1().NetworkPolicies(ns.Name).List(ctx, metav1.ListOptions{})
		if policies == nil || len(policies.Items) == 0 {
			findings = append(findings, engine.Finding{
				ID:        "NET-001",
				Category:  "Network Policies",
				Rule:      "network-policy-exists",
				Severity:  engine.SeverityCritical,
				Resource:  ns.Name,
				Namespace: ns.Name,
				Kind:      "Namespace",
				Message:   fmt.Sprintf("Namespace '%s' has no NetworkPolicy defined — all traffic allowed", ns.Name),
				Remediation: "Create ingress and egress NetworkPolicies to restrict traffic",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

func checkDefaultDenyPolicy(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	namespaces := listNamespaces(ctx, client, namespace)

	for _, ns := range namespaces {
		policies, _ := client.NetworkingV1().NetworkPolicies(ns.Name).List(ctx, metav1.ListOptions{})
		if policies == nil {
			continue
		}

		hasDefaultDeny := false
		for _, policy := range policies.Items {
			// A default deny policy selects all pods and has empty ingress/egress
			if policy.Spec.PodSelector.Size() == 0 {
				hasIngress := false
				hasEgress := false
				for _, pt := range policy.Spec.PolicyTypes {
					if pt == networkingv1.PolicyTypeIngress {
						hasIngress = true
					}
					if pt == networkingv1.PolicyTypeEgress {
						hasEgress = true
					}
				}
				if hasIngress && len(policy.Spec.Ingress) == 0 {
					hasDefaultDeny = true
				}
				if hasEgress && len(policy.Spec.Egress) == 0 {
					hasDefaultDeny = true
				}
			}
		}

		if !hasDefaultDeny && len(policies.Items) > 0 {
			findings = append(findings, engine.Finding{
				ID:        "NET-002",
				Category:  "Network Policies",
				Rule:      "default-deny-policy",
				Severity:  engine.SeverityWarning,
				Resource:  ns.Name,
				Namespace: ns.Name,
				Kind:      "Namespace",
				Message:   fmt.Sprintf("Namespace '%s' has NetworkPolicies but no default-deny policy", ns.Name),
				Remediation: "Create a default-deny NetworkPolicy that selects all pods with empty ingress/egress rules",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

func checkServiceTypes(ctx context.Context, client kubernetes.Interface, namespace string) []engine.Finding {
	var findings []engine.Finding
	services := listServices(ctx, client, namespace)

	for _, svc := range services {
		if svc.Spec.Type == corev1.ServiceTypeNodePort || svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			findings = append(findings, engine.Finding{
				ID:        "NET-003",
				Category:  "Network Policies",
				Rule:      "service-type-check",
				Severity:  engine.SeverityWarning,
				Resource:  fmt.Sprintf("%s/%s", svc.Namespace, svc.Name),
				Namespace: svc.Namespace,
				Kind:      "Service",
				Message:   fmt.Sprintf("Service '%s' uses %s type — exposes traffic externally", svc.Name, svc.Spec.Type),
				Remediation: "Consider using ClusterIP with Ingress controller for better security",
				Timestamp: time.Now(),
			})
		}
	}
	return findings
}

// ============================================================
// HELPER FUNCTIONS
// ============================================================

var systemNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
}

func listPods(ctx context.Context, client kubernetes.Interface, namespace string) []corev1.Pod {
	var allPods []corev1.Pod

	if namespace != "" {
		pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if err != nil || pods == nil {
			return nil
		}
		return pods.Items
	}

	pods, err := client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil || pods == nil {
		return nil
	}

	for _, pod := range pods.Items {
		if !systemNamespaces[pod.Namespace] {
			allPods = append(allPods, pod)
		}
	}
	return allPods
}

func listDeployments(ctx context.Context, client kubernetes.Interface, namespace string) []appsv1.Deployment {
	var allDeploys []appsv1.Deployment

	if namespace != "" {
		deploys, err := client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if err != nil || deploys == nil {
			return nil
		}
		return deploys.Items
	}

	deploys, err := client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil || deploys == nil {
		return nil
	}

	for _, d := range deploys.Items {
		if !systemNamespaces[d.Namespace] {
			allDeploys = append(allDeploys, d)
		}
	}
	return allDeploys
}

func listNamespaces(ctx context.Context, client kubernetes.Interface, namespace string) []corev1.Namespace {
	if namespace != "" {
		return []corev1.Namespace{{ObjectMeta: metav1.ObjectMeta{Name: namespace}}}
	}

	nsList, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil || nsList == nil {
		return nil
	}

	var result []corev1.Namespace
	for _, ns := range nsList.Items {
		if !systemNamespaces[ns.Name] {
			result = append(result, ns)
		}
	}
	return result
}

func listServices(ctx context.Context, client kubernetes.Interface, namespace string) []corev1.Service {
	var allSvcs []corev1.Service

	if namespace != "" {
		svcs, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if err != nil || svcs == nil {
			return nil
		}
		return svcs.Items
	}

	svcs, err := client.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil || svcs == nil {
		return nil
	}

	for _, svc := range svcs.Items {
		if !systemNamespaces[svc.Namespace] {
			allSvcs = append(allSvcs, svc)
		}
	}
	return allSvcs
}

// Suppress unused import warnings — these are used in function signatures
var (
	_ = (*appsv1.Deployment)(nil)
	_ = (*networkingv1.NetworkPolicy)(nil)
)
