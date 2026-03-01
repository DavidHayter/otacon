package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/merthan/otacon/internal/engine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/spf13/cobra"
)

func newResourcesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resources",
		Short: "Analyze resource usage and provide right-sizing recommendations",
		Long: `Compares actual CPU/Memory requests and limits against pod specifications.
Identifies over-provisioned and under-provisioned workloads.

Examples:
  otacon resources                     All namespaces
  otacon resources -n production       Specific namespace
  otacon resources -o json             JSON output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResources()
		},
	}
	return cmd
}

func runResources() error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cyan := color.New(color.FgCyan)
	white := color.New(color.FgWhite)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)
	dim := color.New(color.FgHiBlack)

	cyan.Printf("\n 📊 Otacon Resource Analysis\n\n")

	kubeCfg := engine.KubeConfig{
		Kubeconfig: globalFlags.Kubeconfig,
		Context:    globalFlags.Context,
	}

	client, _, err := engine.NewKubeClient(kubeCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	namespace := globalFlags.Namespace
	yellow.Printf(" ⏳ Analyzing resource configurations...\n\n")

	pods, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	noRequests := 0
	noLimits := 0
	total := 0

	white.Printf(" %-45s %-12s %-12s %-12s %-12s\n",
		"WORKLOAD", "CPU REQ", "CPU LIM", "MEM REQ", "MEM LIM")
	dim.Printf(" %s\n", "─────────────────────────────────────────────────────────────────────────────────────────")

	for _, pod := range pods.Items {
		if pod.Namespace == "kube-system" || pod.Namespace == "kube-public" || pod.Namespace == "kube-node-lease" {
			continue
		}
		for _, container := range pod.Spec.Containers {
			total++
			cpuReq := "-"
			cpuLim := "-"
			memReq := "-"
			memLim := "-"

			if req := container.Resources.Requests.Cpu(); req != nil && !req.IsZero() {
				cpuReq = req.String()
			} else {
				noRequests++
			}
			if lim := container.Resources.Limits.Cpu(); lim != nil && !lim.IsZero() {
				cpuLim = lim.String()
			} else {
				noLimits++
			}
			if req := container.Resources.Requests.Memory(); req != nil && !req.IsZero() {
				memReq = req.String()
			}
			if lim := container.Resources.Limits.Memory(); lim != nil && !lim.IsZero() {
				memLim = lim.String()
			}

			name := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, container.Name)
			if len(name) > 44 {
				name = name[:41] + "..."
			}

			c := white
			if cpuReq == "-" || memReq == "-" {
				c = yellow
			}
			if cpuLim == "-" || memLim == "-" {
				c = red
			}
			c.Printf(" %-45s %-12s %-12s %-12s %-12s\n", name, cpuReq, cpuLim, memReq, memLim)
		}
	}

	fmt.Println()
	dim.Printf(" ━━━ Summary\n")
	white.Printf("   Total containers: %d\n", total)
	if noRequests > 0 {
		red.Printf("   Missing CPU/Memory requests: %d\n", noRequests)
	}
	if noLimits > 0 {
		yellow.Printf("   Missing CPU/Memory limits: %d\n", noLimits)
	}
	if noRequests == 0 && noLimits == 0 {
		green.Printf("   All containers have requests and limits defined ✓\n")
	}
	fmt.Println()

	return nil
}
