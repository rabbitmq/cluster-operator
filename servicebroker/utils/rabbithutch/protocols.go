package rabbithutch

func (r *rabbitHutch) ProtocolPorts() (map[string]int, error) {
	protocolPorts, err := r.client.ProtocolPorts()
	if err != nil {
		return nil, err
	}

	result := make(map[string]int)
	for protocol, port := range protocolPorts {
		result[protocol] = int(port)
	}

	return result, nil
}
