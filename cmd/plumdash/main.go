package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"chameth.com/goplum/api"
	xhttp "chameth.com/x/http"
	"github.com/csmith/envflag/v2"
	"github.com/csmith/slogflags"
)

var (
	listen = flag.String("listen", ":8080", "Address to serve the dashboard on")
	server = flag.String("server", "localhost:7586", "Address of the GoPlum API")
	caCert = flag.String("ca-cert", "ca.crt", "Path to the certificate of the certificate authority for the API")
	cert   = flag.String("cert", "client.crt", "Path to the client certificate to use for the API")
	key    = flag.String("key", "client.key", "Path to the client key to use for the API")
)

func main() {
	envflag.Parse()
	slogflags.Logger(slogflags.WithSetDefault(true))

	if err := run(); err != nil {
		slog.Error("plumdash failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	conn, err := api.Connect(*server, *cert, *key, *caCert)
	if err != nil {
		return fmt.Errorf("unable to connect to API: %w", err)
	}
	defer conn.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	slog.Info("Serving dashboard", "address", *listen, "server", *server)
	return xhttp.ListenAndServe(
		ctx,
		&http.Server{
			Addr:    *listen,
			Handler: newDashboard(api.NewGoPlumClient(conn)),
		},
	)
}
