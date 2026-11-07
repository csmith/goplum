package main

import (
	"fmt"

	"chameth.com/goplum/api"
	"github.com/spf13/cobra"
)

var client api.GoPlumClient

func ConnectToApi(_ *cobra.Command, _ []string) error {
	if err := LoadConfig(); err != nil {
		return fmt.Errorf("unable to load config: %v", err)
	}

	conn, err := api.Connect(
		config.Server,
		config.Certificates.CertPath,
		config.Certificates.KeyPath,
		config.Certificates.CaCertPath,
	)
	if err != nil {
		return fmt.Errorf("unable to connect to API: %v", err)
	}

	client = api.NewGoPlumClient(conn)
	return nil
}
