package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd(version, commit, buildDate string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print NerveGate version details",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("NerveGate Version:    v%s\n", version)
			fmt.Printf("Git Commit:           %s\n", commit)
			fmt.Printf("Build Date:           %s\n", buildDate)
			fmt.Printf("Runtime OS/Arch:      linux/amd64\n")
		},
	}
}
