package binding

func (b Builder) firstHostname() string {
	return b.Hostnames[0]
}

func (b Builder) amqpScheme() string {
	if b.TLS {
		return "amqps"
	}
	return "amqp"
}
