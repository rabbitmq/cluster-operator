package internal

import (
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
)

func GenerateTopicPermissions(p *rabbitmqv1beta1.TopicPermission) rabbithole.TopicPermissions {
	return rabbithole.TopicPermissions{
		Read:     p.Spec.Permissions.Read,
		Write:    p.Spec.Permissions.Write,
		Exchange: p.Spec.Permissions.Exchange,
	}
}
