package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var (
	rootCommand = &cobra.Command{
		Use:   "plumctl",
		Short: "plumctl controls a remote GoPlum instance",
	}
)

func main() {
	if err := rootCommand.Execute(); err != nil {
		bail("Error executing command: %v", err)
	}
}

func bail(format string, args ...interface{}) {
	fmt.Printf(fmt.Sprintf("%s\n", format), args)
	os.Exit(1)
}
