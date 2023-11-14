package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ellis-chen/fa/internal"
	"github.com/ellis-chen/fa/internal/backup"
	"github.com/spf13/cobra"
)

var (
	dumpConfigAsFile bool
)

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump anything releated to fa",
	Run: func(cmd *cobra.Command, _ []string) {
		_ = cmd.Usage()
	},
}

var dumpConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Dump config of fa",
	Run: func(cmd *cobra.Command, _ []string) {
		conf := internal.LoadConf()
		backupConf := backup.LoadCfg()

		fmt.Fprintln(cmd.OutOrStdout(), "---")
		err := internal.DumpConf(conf, cmd.OutOrStdout())
		if err != nil {
			os.Exit(1)
		}
		err = internal.DumpConf(backupConf, cmd.OutOrStdout())
		if err != nil {
			os.Exit(1)
		}

		if dumpConfigAsFile {
			err = ioutil.WriteFile(".fa.yaml", []byte(fmt.Sprintf("%s%s", conf, backupConf)), 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "err output config: %v", err)
			}
		}
	},
}

func init() {
	dumpCmd.AddCommand(dumpConfigCmd)
	RootCmd.AddCommand(dumpCmd)

	flags := dumpConfigCmd.Flags()
	flags.BoolVarP(&dumpConfigAsFile, "dump", "d", false, "dump config to file( .fa.yaml )")
}
