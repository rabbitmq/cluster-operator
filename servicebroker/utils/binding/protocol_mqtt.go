package binding

import (
	"fmt"
	"strings"
)

// addMQTTProtocol takes 'tls' as a parameter because we may need to generate
// an MQTT protocol block for non-TLS MQTT even if TLS is globally enabled
func (b Builder) addMQTTProtocol(port int, tls bool) protocol {
	return protocol{
		Username:  strings.Join([]string{b.VHost, b.Username}, ":"),
		Password:  b.Password,
		Hostname:  b.firstHostname(),
		Hostnames: b.Hostnames,
		URI:       b.uriForMQTT(b.firstHostname(), port, tls),
		URIs:      b.urisForMQTT(port, tls),
		Port:      port,
		TLS:       tls,
	}
}

func (b Builder) uriForMQTT(hostname string, port int, tls bool) string {
	scheme := "mqtt"
	if tls {
		scheme = "mqtt+ssl"
	}
	username := strings.Join([]string{b.VHost, b.Username}, "%3A")
	return fmt.Sprintf("%s://%s:%s@%s:%d", scheme, username, b.Password, hostname, port)
}

func (b Builder) urisForMQTT(port int, tls bool) []string {
	var uris []string
	for _, hostname := range b.Hostnames {
		uris = append(uris, b.uriForMQTT(hostname, port, tls))
	}
	return uris
}
