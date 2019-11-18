package resource

import (
	"fmt"

	rabbitmqv1beta1 "github.com/pivotal/rabbitmq-for-kubernetes/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RabbitmqCluster struct {
	Instance                 *rabbitmqv1beta1.RabbitmqCluster
	ServiceAnnotations       map[string]string
	ServiceType              string
	StatefulSetConfiguration StatefulSetConfiguration
}

func (cluster *RabbitmqCluster) Resources() (resources []runtime.Object, err error) {
	serverConf := cluster.ServerConfigMap()
	resources = append(resources, serverConf)

	ingressService := cluster.IngressService()
	resources = append(resources, ingressService)

	headlessService := cluster.HeadlessService()
	resources = append(resources, headlessService)

	adminSecret, err := cluster.AdminSecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate admin secret: %v ", err)
	}
	resources = append(resources, adminSecret)

	erlangCookie, err := cluster.ErlangCookie()
	if err != nil {
		return nil, fmt.Errorf("failed to generate erlang cookie: %v ", err)
	}
	resources = append(resources, erlangCookie)

	serviceAccount := cluster.ServiceAccount()
	resources = append(resources, serviceAccount)

	role := cluster.Role()
	resources = append(resources, role)

	roleBinding := cluster.RoleBinding()
	resources = append(resources, roleBinding)

	return resources, nil
}
