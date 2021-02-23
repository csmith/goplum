package main

import (
	"context"
	"fmt"
	"github.com/csmith/goplum/api"
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
			fmt.Printf("Unable to stream results: %v\n", err)
			return
		}

		for {
			res, err := resultClient.Recv()
			if err != nil {
				fmt.Printf("Unable to stream results: %v\n", err)
				return
			}
			fmt.Printf("%v\n", res)
		}
	},
}

func init() {
	rootCommand.AddCommand(resultsCommand)
}
