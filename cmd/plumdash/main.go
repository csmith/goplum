package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chameth.com/goplum/api"
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

	srv := &http.Server{
		Addr:              *listen,
		Handler:           newDashboard(api.NewGoPlumClient(conn)),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	errs := make(chan error, 1)
	go func() {
		slog.Info("Serving dashboard", "address", *listen, "server", *server)
		errs <- srv.ListenAndServe()
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
