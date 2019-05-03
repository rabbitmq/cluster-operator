package binding

import (
	"fmt"
)

// addSTOMPProtocol takes 'tls' as a parameter because we may need to generate
// an STOMP protocol block for non-TLS STOMP even if TLS is globally enabled
func (b Builder) addSTOMPProtocol(port int, tls bool) protocol {
	return protocol{
		Username:  b.Username,
		Password:  b.Password,
		Hostname:  b.firstHostname(),
		Hostnames: b.Hostnames,
		URI:       b.uriForSTOMP(b.firstHostname(), port, tls),
		URIs:      b.urisForSTOMP(port, tls),
		Port:      port,
		TLS:       tls,
		VHost:     b.VHost,
	}
}

func (b Builder) uriForSTOMP(hostname string, port int, tls bool) string {
	scheme := "stomp"
	if tls {
		scheme = "stomp+ssl"
	}
	return fmt.Sprintf("%s://%s:%s@%s:%d", scheme, b.Username, b.Password, hostname, port)
}

func (b Builder) urisForSTOMP(port int, tls bool) []string {
	var uris []string
	for _, hostname := range b.Hostnames {
		uris = append(uris, b.uriForSTOMP(hostname, port, tls))
	}
	return uris
}
