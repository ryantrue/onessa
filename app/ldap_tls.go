package app

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"strings"

	"github.com/ryantrue/onessa/internal/logging"
)

func ldapTLSConfig() *tls.Config {
	c := getConfig()
	cfg := &tls.Config{InsecureSkipVerify: c.LDAPTLSInsecureSkipVerify}

	caPath := strings.TrimSpace(c.LDAPCAFile)
	if caPath == "" {
		return cfg
	}

	pem, err := os.ReadFile(caPath)
	if err != nil {
		logging.Warnf("LDAP_CA_FILE read error (%s): %v", caPath, err)
		return cfg
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if ok := pool.AppendCertsFromPEM(pem); !ok {
		logging.Warnf("LDAP_CA_FILE (%s): no certs appended", caPath)
	}

	cfg.RootCAs = pool
	return cfg
}
