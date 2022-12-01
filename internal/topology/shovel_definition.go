package internal

import (
	rabbithole "github.com/michaelklishin/rabbit-hole/v2"
	rabbitmqv1beta1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"strings"
)

func GenerateShovelDefinition(s *rabbitmqv1beta1.Shovel, srcUri, destUri string) rabbithole.ShovelDefinition {
	return rabbithole.ShovelDefinition{
		SourceURI:                        strings.Split(srcUri, ","),
		DestinationURI:                   strings.Split(destUri, ","),
		AckMode:                          s.Spec.AckMode,
		AddForwardHeaders:                s.Spec.AddForwardHeaders,
		DeleteAfter:                      rabbithole.DeleteAfter(s.Spec.DeleteAfter),
		DestinationAddForwardHeaders:     s.Spec.DestinationAddForwardHeaders,
		DestinationAddTimestampHeader:    s.Spec.DestinationAddTimestampHeader,
		DestinationAddress:               s.Spec.DestinationAddress,
		DestinationApplicationProperties: s.Spec.DestinationApplicationProperties,
		DestinationExchange:              s.Spec.DestinationExchange,
		DestinationExchangeKey:           s.Spec.DestinationExchangeKey,
		DestinationProperties:            s.Spec.DestinationProperties,
		DestinationProtocol:              s.Spec.DestinationProtocol,
		DestinationPublishProperties:     s.Spec.DestinationPublishProperties,
		DestinationQueue:                 s.Spec.DestinationQueue,
		PrefetchCount:                    s.Spec.PrefetchCount,
		ReconnectDelay:                   s.Spec.ReconnectDelay,
		SourceAddress:                    s.Spec.SourceAddress,
		SourceDeleteAfter:                rabbithole.DeleteAfter(s.Spec.SourceDeleteAfter),
		SourceExchange:                   s.Spec.SourceExchange,
		SourceExchangeKey:                s.Spec.SourceExchangeKey,
		SourcePrefetchCount:              s.Spec.SourcePrefetchCount,
		SourceProtocol:                   s.Spec.SourceProtocol,
		SourceQueue:                      s.Spec.SourceQueue,
	}

}
