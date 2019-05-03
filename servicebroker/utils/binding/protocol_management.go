package binding

import (
	"fmt"
)

func (b Builder) addMgmtProtocol() protocol {
	return protocol{
		Username:  b.Username,
		Password:  b.Password,
		Hostname:  b.firstHostname(),
		Hostnames: b.Hostnames,
		URI:       b.uriForManagement(b.firstHostname(), 15672),
		URIs:      b.urisForManagement(15672),
		Port:      15672,
		TLS:       false,
		Path:      "/api/",
	}
}

func (b Builder) addMgmtProtocolTLS() protocol {
	return protocol{
		Username:  b.Username,
		Password:  b.Password,
		Hostname:  b.firstHostname(),
		Hostnames: b.Hostnames,
		URI:       b.uriForManagement(b.firstHostname(), 15672),
		URIs:      b.urisForManagement(15672),
		Port:      15672,
		TLS:       false,
		Path:      "/api/",
	}
}

func (b Builder) uriForManagement(hostname string, port int) string {
	return fmt.Sprintf("http://%s:%s@%s:%d/api/", b.Username, b.Password, hostname, port)
}

func (b Builder) urisForManagement(port int) []string {
	var uris []string
	for _, hostname := range b.Hostnames {
		uris = append(uris, b.uriForManagement(hostname, port))
	}
	return uris
}
