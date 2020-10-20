package main

import (
	"context"
	"fmt"
	"github.com/csmith/goplum/api"
	"github.com/spf13/cobra"
)

var suspendCommand = &cobra.Command{
	Use: "suspend <name>",
	Short: "Suspend a check",
	Args: cobra.ExactArgs(1),
	PreRunE: ConnectToApi,
	Run: func(cmd *cobra.Command, args []string) {
		check, err := client.SuspendCheck(context.Background(), &api.CheckName{Name: args[0]})
		if err != nil {
			fmt.Printf("Unable to suspend check: %v\n", err)
			return
		}
		fmt.Printf("Suspended check %s.\n", check.Name)
	},
}

func init() {
	rootCommand.AddCommand(suspendCommand)
}
