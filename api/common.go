package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

var (
	ServiceDesc = &_GoPlum_serviceDesc
)

func LoadCertificates(localCert, localKey, caCert string) ([]tls.Certificate, *x509.CertPool, error) {
	certificate, err := tls.LoadX509KeyPair(localCert, localKey)
	if err != nil {
		return nil, nil, err
	}

	certPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile(caCert)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read ca cert: %v", err)
	}

	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		return nil, nil, fmt.Errorf("failed to append ca cert")
	}

	return []tls.Certificate{certificate}, certPool, nil
}
