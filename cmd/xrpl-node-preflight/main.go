// Command xrpl-node-preflight checks whether a Linux host and an optional
// xrpld configuration are ready to run an XRP Ledger node or validator.
package main

import (
	"os"

	"github.com/LionelTrichet/xrpl-node-preflight/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
