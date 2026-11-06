package main

import (
	"context"
	"fmt"

	"chameth.com/goplum/api"
	"github.com/spf13/cobra"
)

var resultsCommand = &cobra.Command{
	Use:     "results",
	Short:   "Streams check results from the server",
	Args:    cobra.NoArgs,
	PreRunE: ConnectToApi,
	Run: func(cmd *cobra.Command, args []string) {
		resultClient, err := client.Results(context.Background(), &api.Empty{})
		if err != nil {
			bailf("Unable to stream results: %v", err)
		}

		for {
			res, err := resultClient.Recv()
			if err != nil {
				bailf("Unable to stream results: %v", err)
			}
			fmt.Printf("%v\n", res)
		}
	},
}

func init() {
	rootCommand.AddCommand(resultsCommand)
}
