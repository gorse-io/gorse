// Copyright 2024 gorse Project Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"crypto/tls"
	"crypto/x509"
	"github.com/juju/errors"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/security/advancedtls"
	"os"
)

type TLSConfig struct {
	SSLCA   string
	SSLCert string
	SSLKey  string
}

func NewServerCreds(o *TLSConfig) (credentials.TransportCredentials, error) {
	// Load certification authority
	ca := x509.NewCertPool()
	pem, err := os.ReadFile(o.SSLCA)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if !ca.AppendCertsFromPEM(pem) {
		return nil, errors.New("failed to append certificate")
	}
	// Load certification
	certificate, err := tls.LoadX509KeyPair(o.SSLCert, o.SSLKey)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// Create server credentials
	return advancedtls.NewServerCreds(&advancedtls.Options{
		IdentityOptions: advancedtls.IdentityCertificateOptions{
			Certificates: []tls.Certificate{certificate},
		},
		RootOptions: advancedtls.RootCertificateOptions{
			RootCertificates: ca,
		},
		RequireClientCert: true,
		VerificationType:  advancedtls.CertVerification,
	})
}

func NewClientCreds(o *TLSConfig) (credentials.TransportCredentials, error) {
	// Load certification authority
	ca := x509.NewCertPool()
	pem, err := os.ReadFile(o.SSLCA)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if !ca.AppendCertsFromPEM(pem) {
		return nil, errors.New("failed to append certificate")
	}
	// Load certification
	certificate, err := tls.LoadX509KeyPair(o.SSLCert, o.SSLKey)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// Create client credentials
	return advancedtls.NewClientCreds(&advancedtls.Options{
		IdentityOptions: advancedtls.IdentityCertificateOptions{
			Certificates: []tls.Certificate{certificate},
		},
		RootOptions: advancedtls.RootCertificateOptions{
			RootCertificates: ca,
		},
		VerificationType: advancedtls.CertVerification,
	})
}
