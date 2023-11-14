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
	"context"
	"os"

	"github.com/ellis-chen/fa/global"
	"github.com/ellis-chen/fa/web/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	port int16
	path string
)

// servCmd represents the deploy command
var servCmd = &cobra.Command{
	Use:     "serve",
	Version: RootCmd.Version,
	Short:   "Serve the file access service",
	Long:    `Serve the fa service. Default port in use is 8080 `,
	Run: func(c *cobra.Command, _ []string) {
		if port != 8080 {
			viper.Set("port", port)
		}
		_, exists := os.LookupEnv("FA_STORE_PATH")
		if !exists && path != "." {
			viper.Set("store_path", path)
		}

		ctx := context.WithValue(c.Context(), global.CliVerKey, CliVersion())
		server.Serve(ctx)
	},
}

func init() {
	RootCmd.AddCommand(servCmd)

	servCmd.Flags().Int16VarP(&port, "port", "p", 8080, "Port the fa service listen to (can also be provided by FA_PORT env var)")
	servCmd.Flags().StringVarP(&path, "path", "P", ".", "Path the fa service store the file(can also be provided by FA_STORE_PATH env var)")
}
