package api

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type FactValue = isFact_Value

// Connect creates a client for the API at address ("host:port"), using mutual
// TLS with the given client certificate, key and CA certificate.
func Connect(address, certPath, keyPath, caPath string) (*grpc.ClientConn, error) {
	certs, pool, err := LoadCertificates(certPath, keyPath, caPath)
	if err != nil {
		return nil, fmt.Errorf("unable to load certificates: %w", err)
	}

	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	return grpc.NewClient(
		address,
		grpc.WithTransportCredentials(
			credentials.NewTLS(
				&tls.Config{
					ServerName:   host,
					Certificates: certs,
					RootCAs:      pool,
					MinVersion:   tls.VersionTLS13,
				},
			),
		),
	)
}

func LoadCertificates(localCert, localKey, caCert string) ([]tls.Certificate, *x509.CertPool, error) {
	certificate, err := tls.LoadX509KeyPair(localCert, localKey)
	if err != nil {
		return nil, nil, err
	}

	certPool := x509.NewCertPool()
	bs, err := os.ReadFile(caCert)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read ca cert: %v", err)
	}

	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		return nil, nil, errors.New("failed to append ca cert")
	}

	return []tls.Certificate{certificate}, certPool, nil
}
