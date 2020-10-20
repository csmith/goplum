package main

import (
	"context"
	"crypto/tls"
	"flag"
	"github.com/csmith/goplum/api"
	"github.com/kouhin/envflag"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"log"
	"net"
)

var (
	apiAddress = flag.String("api-address", "localhost:7586", "Address of the GoPlum API")
)

func main() {
	if err := envflag.Parse(); err != nil {
		panic(err)
	}

	certs, pool, err := api.LoadCertificates()
	if err != nil {
		log.Fatalf("Couldn't load certificates: %v", err)
	}

	host, _, err := net.SplitHostPort(*apiAddress)
	if err != nil {
		log.Fatalf("Unable to parse API address: %v", err)
	}

	transportCreds := credentials.NewTLS(&tls.Config{
		ServerName:   host,
		Certificates: certs,
		RootCAs:      pool,
	})
	conn, err := grpc.Dial(*apiAddress, grpc.WithTransportCredentials(transportCreds))
	if err != nil {
		log.Fatalf("Unable to dial server: %v", err)
	}
	defer conn.Close()

	client := api.NewGoPlumClient(conn)
	rc, err := client.Results(context.Background(), &api.Empty{})
	if err != nil {
		log.Fatalf("Unable to retrieve check results: %v", err)
	}

	for {
		result, err := rc.Recv()
		if err != nil {
			log.Fatalf("Error receiving result: %v", err)
		}

		log.Printf("Check '%s' executed with result '%s'", result.GetCheck(), api.Status_name[int32(result.GetResult())])
	}
}
