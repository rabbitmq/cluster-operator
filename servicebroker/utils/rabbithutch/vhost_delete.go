package rabbithutch

func (b *rabbitHutch) VHostDelete(vhost string) error {
	return validateResponse(b.client.DeleteVhost(vhost))
}
