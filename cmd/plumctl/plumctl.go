package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	rootCommand = &cobra.Command{
		Use:   "plumctl",
		Short: "plumctl controls a remote GoPlum instance",
	}
)

func main() {
	if err := rootCommand.Execute(); err != nil {
		bailf("Error executing command: %v", err)
	}
}

func bailf(format string, args ...any) {
	fmt.Printf(format+"\n", args)
	os.Exit(1)
}
