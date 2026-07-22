package main

import (
	"os"

	"github.com/marcoarnulfo/clickup-cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
