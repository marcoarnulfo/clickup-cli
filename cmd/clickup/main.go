package main

import (
	"fmt"
	"os"

	"github.com/marcoarnulfo/clickup-cli/internal/cli"
)

func main() {
	code := cli.Execute()
	fmt.Fprintln(os.Stderr, "warning: 'clickup' is deprecated; install and use 'clup' instead.")
	os.Exit(code)
}
