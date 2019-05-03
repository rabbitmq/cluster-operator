package binding

import (
	"encoding/json"
	"fmt"
)

type binding struct {
	DashboardURL string    `json:"dashboard_url"`
	Username     string    `json:"username"`
	Password     string    `json:"password"`
	Hostname     string    `json:"hostname"`
	Hostnames    []string  `json:"hostnames"`
	HTTPAPIURI   string    `json:"http_api_uri"`
	HTTPAPIURIs  []string  `json:"http_api_uris"`
	URI          string    `json:"uri"`
	URIs         []string  `json:"uris"`
	VHost        string    `json:"vhost"`
	TLS          bool      `json:"ssl"`
	Protocols    protocols `json:"protocols"`
}

type Builder struct {
	MgmtDomain    string
	Hostnames     []string
	VHost         string
	Username      string
	Password      string
	TLS           bool
	ProtocolPorts map[string]int // key=protocol, value=port, e.g. "amqp": 4567
}

func (b Builder) Build() (output interface{}, err error) {
	bind := binding{
		VHost:        b.VHost,
		Username:     b.Username,
		Password:     b.Password,
		DashboardURL: b.dashboardURL(),
		Hostname:     b.firstHostname(),
		Hostnames:    b.Hostnames,
		HTTPAPIURI:   b.httpapiuriForBinding(),
		HTTPAPIURIs:  b.httpapiurisForBinding(),
		URI:          b.uriForBinding(b.firstHostname()),
		URIs:         b.urisForBinding(),
		TLS:          b.TLS,
		Protocols:    b.protocols(),
	}

	bytes, err := json.Marshal(bind)
	if err != nil {
		return output, err
	}

	err = json.Unmarshal(bytes, &output)
	return output, err
}

func (b Builder) dashboardURL() string {
	return fmt.Sprintf("http://%s/#/login/%s/%s", b.MgmtDomain, b.Username, b.Password)
}

func (b Builder) uriForBinding(hostname string) string {
	return fmt.Sprintf("%s://%s:%s@%s/%s", b.amqpScheme(), b.Username, b.Password, hostname, b.VHost)
}

func (b Builder) urisForBinding() []string {
	var uris []string
	for _, hostname := range b.Hostnames {
		uris = append(uris, b.uriForBinding(hostname))
	}
	return uris
}

func (b Builder) httpapiuriForBinding() string {
	return fmt.Sprintf("http://%s:%s@%s/api/", b.Username, b.Password, b.MgmtDomain)
}

func (b Builder) httpapiurisForBinding() []string {
	return []string{b.httpapiuriForBinding()}
}
