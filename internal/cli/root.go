package cli

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

const banner = `
  ██████╗ ████████╗ █████╗  ██████╗ ██████╗ ███╗   ██╗
 ██╔═══██╗╚══██╔══╝██╔══██╗██╔════╝██╔═══██╗████╗  ██║
 ██║   ██║   ██║   ███████║██║     ██║   ██║██╔██╗ ██║
 ██║   ██║   ██║   ██╔══██║██║     ██║   ██║██║╚██╗██║
 ╚██████╔╝   ██║   ██║  ██║╚██████╗╚██████╔╝██║ ╚████║
  ╚═════╝    ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚═════╝ ╚═╝  ╚═══╝`

// GlobalFlags holds flags available to all commands
type GlobalFlags struct {
	Kubeconfig string
	Context    string
	Namespace  string
	OutputFmt  string
	Verbose    bool
	NoColor    bool
}

var globalFlags GlobalFlags

func NewRootCommand(info BuildInfo) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "otacon",
		Short: "Intelligent Kubernetes Diagnostics & Audit Platform",
		Long: fmt.Sprintf(`%s
  %s

  Otacon is an event-driven intelligence platform for Kubernetes.
  It correlates events, audits best practices, and delivers
  noise-free, actionable diagnostics.

  Quick Start:
    otacon scan                    Full cluster health scan
    otacon scan -n production      Namespace-specific scan
    otacon audit                   Best practice compliance check
    otacon diagnose network        Network diagnostics
    otacon diagnose logs           Log pattern analysis
    otacon diagnose nodes          Node pressure detection
    otacon events --since 1h       Correlated event timeline
    otacon resources               Resource right-sizing`,
			color.CyanString(banner),
			color.HiBlackString("Kubernetes Intelligence Platform %s", info.Version)),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flags
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&globalFlags.Kubeconfig, "kubeconfig", "", "Path to kubeconfig file (default: ~/.kube/config)")
	pf.StringVar(&globalFlags.Context, "context", "", "Kubernetes context to use")
	pf.StringVarP(&globalFlags.Namespace, "namespace", "n", "", "Target namespace (default: all namespaces)")
	pf.StringVarP(&globalFlags.OutputFmt, "output", "o", "table", "Output format: table, json, yaml, wide")
	pf.BoolVarP(&globalFlags.Verbose, "verbose", "v", false, "Enable verbose output")
	pf.BoolVar(&globalFlags.NoColor, "no-color", false, "Disable colored output")

	// Register subcommands
	rootCmd.AddCommand(newVersionCommand(info))
	rootCmd.AddCommand(newScanCommand())
	rootCmd.AddCommand(newAuditCommand())
	rootCmd.AddCommand(newDiagnoseCommand())
	rootCmd.AddCommand(newResourcesCommand())
	rootCmd.AddCommand(newEventsCommand())
	rootCmd.AddCommand(newGuardianCommand())
	rootCmd.AddCommand(newCompletionCommand())

	return rootCmd
}
