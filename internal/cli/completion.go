package cli

import (
	"os"

	"github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for Otacon.

To load completions:

Bash:
  $ source <(otacon completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ otacon completion bash > /etc/bash_completion.d/otacon
  # macOS:
  $ otacon completion bash > $(brew --prefix)/etc/bash_completion.d/otacon

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  # To load completions for each session, execute once:
  $ otacon completion zsh > "${fpath[1]}/_otacon"

Fish:
  $ otacon completion fish | source
  # To load completions for each session, execute once:
  $ otacon completion fish > ~/.config/fish/completions/otacon.fish

PowerShell:
  PS> otacon completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> otacon completion powershell > otacon.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}
	return cmd
}
