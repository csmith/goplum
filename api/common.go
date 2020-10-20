package api

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
)

var (
	caCert    = flag.String("ca-cert", "ca.crt", "Path to the certificate of the certificate authority for the API")
	localCert = flag.String("cert", "goplum.crt", "Path to the certificate to use for the API")
	localKey  = flag.String("key", "goplum.key", "Path to the key to use for the API")

	ServiceDesc = &_GoPlum_serviceDesc
)

// LoadCertificates loads the local and CA certificates.
func LoadCertificates() ([]tls.Certificate, *x509.CertPool, error) {
	certificate, err := tls.LoadX509KeyPair(*localCert, *localKey)
	if err != nil {
		return nil, nil, err
	}

	certPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile(*caCert)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read ca cert: %v", err)
	}

	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		return nil, nil, fmt.Errorf("failed to append ca cert")
	}

	return []tls.Certificate{certificate}, certPool, nil
}
