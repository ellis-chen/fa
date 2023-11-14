package cmd

import (
	"context"
	"fmt"

	"github.com/ellis-chen/fa/global"
	"github.com/spf13/cobra"
)

var (
	BuildTime   = ""
	BuildNumber = ""
	BuildBy     = ""
	GitCommit   = ""
	Version     = "1.0.0"
)
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of fa",
	Run: func(cmd *cobra.Command, _ []string) {
		fmt.Fprintf(cmd.OutOrStdout(),
			"Version:    %s\n"+
				"    Git Commit  : %s\n"+
				"    Build Time  : %s\n"+
				"    Build Number: %s\n"+
				"    Build By    : %s\n",
			Version, GitCommit, BuildTime, BuildNumber, BuildBy)
	},
}

func RootCtx() context.Context {
	return context.WithValue(context.TODO(), global.CliVerKey, CliVersion())
}

func CliVersion() string {
	return fmt.Sprintf("%s-%s", Version, GitCommit)
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
