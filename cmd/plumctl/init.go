package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"net"
	"path/filepath"
)

var initCommand = &cobra.Command{
	Use: "init <host:port>",
	Short: "Initialise plumctl for use with a remote instance of GoPlum",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		_, _, err := net.SplitHostPort(args[0])
		if err != nil {
			fmt.Printf("Invalid host: %v\n", err)
			return
		}

		p, err := configPath()
		if err != nil {
			fmt.Printf("Unable to get config path: %v\n", err)
			return
		}

		dir := filepath.Dir(p)
		config = &Config{
			Server: args[0],
			Certificates: Certificates{
				CaCertPath: filepath.Join(dir, "ca.crt"),
				CertPath:   filepath.Join(dir, "client.crt"),
				KeyPath:    filepath.Join(dir, "client.key"),
			},
		}
		if err := SaveConfig(); err != nil {
			fmt.Printf("Unable to save config: %v\n", err)
			return
		}

		fmt.Printf("Config created in %s.\n", dir)
		fmt.Printf("You must provide your CA certificate, client certificate and client private key:\n\n")
		fmt.Printf("\t    CA cert: %s\n", filepath.Join(dir, "ca.crt"))
		fmt.Printf("\tClient cert: %s\n", filepath.Join(dir, "client.crt"))
		fmt.Printf("\t Client key: %s\n", filepath.Join(dir, "client.key"))
		fmt.Printf("\nYou can adjust these paths in the configuration file.\n")
	},
}

func init() {
	rootCommand.AddCommand(initCommand)
}
