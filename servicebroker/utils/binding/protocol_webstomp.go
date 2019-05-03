package binding

import (
	"fmt"
)

func (b Builder) addWebSTOMPProtocol(port int) protocol {
	return protocol{
		Username:  b.Username,
		Password:  b.Password,
		Hostname:  b.firstHostname(),
		Hostnames: b.Hostnames,
		URI:       b.uriForWebSTOMP(b.firstHostname(), port),
		URIs:      b.urisForWebSTOMP(port),
		Port:      port,
		TLS:       false,
		VHost:     b.VHost,
	}
}

func (b Builder) uriForWebSTOMP(hostname string, port int) string {
	scheme := "http/web-stomp"
	return fmt.Sprintf("%s://%s:%s@%s:%d/%s", scheme, b.Username, b.Password, hostname, port, b.VHost)
}

func (b Builder) urisForWebSTOMP(port int) []string {
	var uris []string
	for _, hostname := range b.Hostnames {
		uris = append(uris, b.uriForWebSTOMP(hostname, port))
	}
	return uris
}
