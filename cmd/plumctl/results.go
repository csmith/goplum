package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

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

		if err := printResults(resultClient, os.Stdout); err != nil {
			bailf("Unable to stream results: %v", err)
		}
	},
}

func init() {
	rootCommand.AddCommand(resultsCommand)
}

func printResults(stream api.GoPlum_ResultsClient, out io.Writer) error {
	for {
		res, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%v\n", res); err != nil {
			return err
		}
	}
}
