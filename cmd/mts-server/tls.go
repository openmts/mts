package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

func buildTLSConfig(cfg tlsConfig) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load tls key pair: %w", err)
	}
	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	}
	if cfg.ClientAuth {
		pool, err := loadClientCAPool(cfg.ClientCAFile)
		if err != nil {
			return nil, err
		}
		tlsCfg.ClientCAs = pool
		tlsCfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return tlsCfg, nil
}

func loadClientCAPool(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read client ca: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("%w: client_ca_file has no PEM certificates", errInvalidConfig)
	}
	return pool, nil
}
