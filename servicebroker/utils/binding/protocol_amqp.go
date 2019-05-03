package binding

import "fmt"

func (b Builder) addAMQPProtocol(port int, tls bool) protocol {
	return protocol{
		Username:  b.Username,
		Password:  b.Password,
		VHost:     b.VHost,
		Hostname:  b.firstHostname(),
		Hostnames: b.Hostnames,
		URI:       b.uriForAMQP(b.firstHostname(), port),
		URIs:      b.urisForAMQP(port),
		Port:      port,
		TLS:       tls,
	}
}

func (b Builder) uriForAMQP(hostname string, port int) string {
	return fmt.Sprintf("%s://%s:%s@%s:%d/%s", b.amqpScheme(), b.Username, b.Password, hostname, port, b.VHost)
}

func (b Builder) urisForAMQP(port int) []string {
	var uris []string
	for _, hostname := range b.Hostnames {
		uris = append(uris, b.uriForAMQP(hostname, port))
	}
	return uris
}
