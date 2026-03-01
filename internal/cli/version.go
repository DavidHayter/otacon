package cli

import (
	"fmt"
	"runtime"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func newVersionCommand(info BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			cyan := color.New(color.FgCyan).SprintFunc()
			white := color.New(color.FgWhite).SprintFunc()

			fmt.Printf("%s %s\n", cyan("Otacon"), white(info.Version))
			fmt.Printf("  Commit:    %s\n", info.Commit)
			fmt.Printf("  Built:     %s\n", info.Date)
			fmt.Printf("  Go:        %s\n", runtime.Version())
			fmt.Printf("  Platform:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}
