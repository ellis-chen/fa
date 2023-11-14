package cmd

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/ellis-chen/fa/pkg/jwt"
)

var (
	fromToken, jwtSecret string
)

var miscCmd = &cobra.Command{
	Use:   "misc",
	Short: "misc tools to aid the usage of fa",
	Run: func(cmd *cobra.Command, _ []string) {
		_ = cmd.Usage()
	},
}

var miscGenJwtCmd = &cobra.Command{
	Use:   "gen-jwt",
	Short: "generate a jwt token to help test",
	Example: `
# use arguments to gen jwt
fa misc gen-jwt -t <jwt token> -s <jwt secret>

# use environment to gen jwt
export JWT_TOKEN=<jwt token>
export JWT_SECRET=<jwt secret>
fa misc gen-jwt
	`,
	Run: func(_ *cobra.Command, _ []string) {
		token, exists := os.LookupEnv("JWT_TOKEN")
		if exists {
			fromToken = token
		}

		secret, exists := os.LookupEnv("JWT_SECRET")
		if exists {
			jwtSecret = secret
		}
		logrus.Debugf("generate new jwt token from token [ %s ] and secret [ %s ]", fromToken, jwtSecret)
		token, err := jwt.ExtendJwtExp1Year(fromToken, jwtSecret)
		if err != nil {
			logrus.Errorf("err extend the exp time of token: %v", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, token)
	},
}

func init() {
	miscCmd.AddCommand(miscGenJwtCmd)
	RootCmd.AddCommand(miscCmd)

	flags := miscGenJwtCmd.Flags()
	flags.StringVarP(&fromToken, "token", "t", "original jwt token", "The original possible expired jwt token (can supply via JWT_TOKEN environment variable)")
	flags.StringVarP(&jwtSecret, "secret", "s", "jwt secret", "The jwt secret to encrypt or verify jwt token (can supply via JWT_SECRET environment variable)")
}
