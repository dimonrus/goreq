package goreq

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dimonrus/porterr"
	"golang.org/x/net/http2"
)

// SecureClient Init secure client
func SecureClient(certPath string) (*http.Client, porterr.IError) {
	client := &http.Client{}
	caCert, err := os.ReadFile(filepath.Clean(certPath))
	if err != nil {
		return nil, porterr.New(porterr.PortErrorIO, err.Error())
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)
	client.Transport = &http2.Transport{
		TLSClientConfig: &tls.Config{RootCAs: caCertPool},
	}
	return client, nil
}
