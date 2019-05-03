package binding

type protocol struct {
	Username  string   `json:"username"`
	Password  string   `json:"password"`
	VHost     string   `json:"vhost,omitempty"`
	Hostname  string   `json:"host"`
	Hostnames []string `json:"hosts"`
	URI       string   `json:"uri"`
	URIs      []string `json:"uris"`
	Port      int      `json:"port"`
	TLS       bool     `json:"ssl"`
	Path      string   `json:"path,omitempty"`
}

type protocols map[string]protocol

func (b Builder) protocols() protocols {
	ps := make(protocols)
	for protocol, port := range b.ProtocolPorts {
		switch protocol {
		case "amqp":
			ps["amqp"] = b.addAMQPProtocol(port, false)
		case "amqp/ssl":
			ps["amqp+ssl"] = b.addAMQPProtocol(port, true)
		case "mqtt":
			ps["mqtt"] = b.addMQTTProtocol(port, false)
		case "mqtt/ssl":
			ps["mqtt+ssl"] = b.addMQTTProtocol(port, true)
		case "stomp":
			ps["stomp"] = b.addSTOMPProtocol(port, false)
		case "stomp/ssl":
			ps["stomp+ssl"] = b.addSTOMPProtocol(port, true)
		case "http/web-stomp":
			ps["ws"] = b.addWebSTOMPProtocol(port)
		}
	}
	ps["management"] = b.addMgmtProtocol()
	if b.TLS {
		ps["management+ssl"] = b.addMgmtProtocolTLS()
	}
	return ps
}
