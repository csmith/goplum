package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/csmith/goplum/api"
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

		fmt.Printf("%d checks\n", len(checks.Checks))
		for i := range checks.Checks {
			c := checks.Checks[i]
			var extras []string

			if c.Suspended {
				extras = append(extras, "*SUSPENDED*")
			}

			if !c.Settled {
				extras = append(extras, "[not settled]")
			}

			if c.State != api.Status_GOOD {
				extras = append(extras, fmt.Sprintf("[%s]", api.Status_name[int32(c.State)]))
			}

			fmt.Printf("%d. %s (type: %s) %s\n", i+1, c.Name, c.Type, strings.Join(extras, " "))
		}
	},
}

func init() {
	rootCommand.AddCommand(checksCommand)
}
