package main

import (
	"crypto/tls"

	"golang.org/x/crypto/acme/autocert"
)

// thanks to https://d3void.net/post/acme/

const contactEmail = "yourname@example.com"

func GetTLS(host, cacheDir string) (*tls.Config, error) {
	manager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(cacheDir),
		HostPolicy: autocert.HostWhitelist(host),
		Email:      contactEmail,
	}
	return &tls.Config{GetCertificate: manager.GetCertificate}, nil
}

//tls, err := GetTLS(cfg.TLSHost, cfg.TLSCacheDir)
//if err != nil {
//	return nil, errors.Wrap(err, "Could not get TLS cert")
//}
//creds := credentials.NewTLS(tls)
//server := grpc.NewServer(grpc.Creds(creds))
