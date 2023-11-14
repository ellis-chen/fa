/*
Package cmd for fa

Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ellis-chen/fa/internal"
	"github.com/ellis-chen/fa/internal/logger"
	"github.com/ellis-chen/fa/pkg/iotools"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	cfgFile    string
	verbose    bool
	genManPage bool
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:               "fa",
	Version:           Version,
	Short:             "file access management for compute cluster",
	Long:              `The 'fa' tool is used to provide a union file access tools for ellis-chen product `,
	DisableAutoGenTag: true,
	Example: `
# list help
fa --help

# gen manpage
fa --man
	`,
	Run: func(cmd *cobra.Command, _ []string) { // OnInitialize is called first

		if genManPage {
			now := time.Now()
			header := &doc.GenManHeader{
				Title:   "fa reference manual",
				Section: "1",
				Source:  "Doc generated by <neo.chen@ellis-chentech.com>",
				Date:    &now,
				Manual:  "fa usermanual",
			}
			viper.SetDefault("man_loc", "/tmp")
			err := doc.GenManTree(cmd, header, viper.GetString("man_loc"))
			if err != nil {
				log.Fatal(err)
			}

			return
		}

		_ = cmd.Usage()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(RootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.fa.yaml)")
	RootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose the log")
	RootCmd.Flags().BoolVarP(&genManPage, "man", "m", false, "generate the man page for fa")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	conf := internal.LoadConf(cfgFile)
	err := iotools.EnsureDir(conf.StorePath)
	if err != nil {
		return
	}

	logger.SetLevel(logrus.InfoLevel)
	if verbose || conf.Debug {
		logger.SetLevel(logrus.DebugLevel)
	}
	if conf.Log.TermMode {
		currentPath, _ := os.Getwd()
		os.Setenv("FA_STORE_PATH", currentPath)

		conf.Log.Path = "logs/fa.log"
		_ = iotools.EnsureFile(conf.Log.Path)

		logger.SetFormatter(&logrus.TextFormatter{
			ForceColors:     true,
			DisableColors:   false,
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		})
	} else {
		logger.SetFormatter(&logrus.JSONFormatter{})
	}

	logrus.Debugf("log path is %s", conf.Log.Path)
	l := lumberjack.Logger{
		Filename:   conf.Log.Path,
		MaxSize:    conf.Log.MaxSize, // megabytes
		MaxBackups: conf.Log.MaxBackups,
		MaxAge:     conf.Log.MaxAge, // days
	}

	logger.SetRotateOut(&l)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGHUP)

	go func() {
		for {
			<-c
			logrus.Infof("log rotating")
			err := l.Rotate()
			if err != nil {
				logrus.Errorf("log rotating error: %v", err)
			}
		}
	}()
}
