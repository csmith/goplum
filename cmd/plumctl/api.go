package main

import (
	"crypto/tls"
	"fmt"
	"net"

	"chameth.com/goplum/api"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var client api.GoPlumClient

func ConnectToApi(_ *cobra.Command, _ []string) error {
	if err := LoadConfig(); err != nil {
		return fmt.Errorf("unable to load config: %v", err)
	}

	certs, pool, err := api.LoadCertificates(
		config.Certificates.CertPath,
		config.Certificates.KeyPath,
		config.Certificates.CaCertPath,
	)
	if err != nil {
		return fmt.Errorf("unable to load certificates: %v", err)
	}

	host, _, err := net.SplitHostPort(config.Server)
	if err != nil {
		return fmt.Errorf("invalid address: %v", err)
	}

	transportCreds := credentials.NewTLS(&tls.Config{
		ServerName:   host,
		Certificates: certs,
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS13,
	})
	conn, err := grpc.Dial(config.Server, grpc.WithTransportCredentials(transportCreds))
	if err != nil {
		return fmt.Errorf("unable to dial API: %v", err)
	}

	client = api.NewGoPlumClient(conn)
	return nil
}
