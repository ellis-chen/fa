package main

import (
	"github.com/ellis-chen/fa/cmd"
	"github.com/ellis-chen/fa/web/server"
)

func main() {
	server.Serve(cmd.RootCtx())
}
