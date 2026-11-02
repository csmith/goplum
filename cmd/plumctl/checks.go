package main

import (
	"context"
	"fmt"
	"strings"

	"chameth.com/goplum/api"
	"github.com/spf13/cobra"
)

var checksCommand = &cobra.Command{
	Use:     "checks",
	Short:   "Lists all known checks",
	Args:    cobra.NoArgs,
	PreRunE: ConnectToApi,
	Run: func(cmd *cobra.Command, args []string) {
		checks, err := client.GetChecks(context.Background(), &api.Empty{})
		if err != nil {
			fmt.Printf("Unable to retrieve checks: %v\n", err)
			return
		}

		fmt.Printf("%d checks\n", len(checks.GetChecks()))
		for i := range checks.GetChecks() {
			c := checks.GetChecks()[i]
			var extras []string

			if c.GetSuspended() {
				extras = append(extras, "*SUSPENDED*")
			}

			if !c.GetSettled() {
				extras = append(extras, "[not settled]")
			}

			if c.GetState() != api.Status_GOOD {
				extras = append(extras, fmt.Sprintf("[%s]", api.Status_name[int32(c.GetState())]))
			}

			fmt.Printf("%d. %s (type: %s) %s\n", i+1, c.GetName(), c.GetType(), strings.Join(extras, " "))
		}
	},
}

func init() {
	rootCommand.AddCommand(checksCommand)
}
