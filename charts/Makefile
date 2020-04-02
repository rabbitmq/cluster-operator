# runs the target list by default
.DEFAULT_GOAL = list
.PHONY: list

# Insert a comment starting with '##' after a target, and it will be printed by 'make' and 'make list'
list:
	@echo "The most used targets: \n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# helm setup/configure
DEPLOYMENT_NAME=rabbitmq-operator
CHARTREPO_NAME=dev-pivnet

setup: ## register the dev pivnet registry with helm
	@echo "Registering Dev PivNet registry"
	@helm repo add "${CHARTREPO_NAME}" \
		https://dev.registry.pivotal.io/chartrepo/p-rabbitmq-for-kubernetes \
		--username='$(shell lpassd show "Shared-RabbitMQ for Kubernetes/Pivnet user - shared" --username)' \
		--password='$(shell lpassd show "Shared-RabbitMQ for Kubernetes/Pivnet user - shared" --password)'

CHART_NAME=${CHARTREPO_NAME}/rabbitmq-operator
destroy: ## helm uninstall operator chart
	helm -n default uninstall "${DEPLOYMENT_NAME}"

deploy: ## helm install latest version of operator chart hosted in Dev PivNet
	helm -n default install "${DEPLOYMENT_NAME}" "${CHART_NAME}" \
		--set global.imageUsername='$(shell lpassd show "Shared-RabbitMQ for Kubernetes/pivnet-dev-registry-ci" --notes | jq -r .name)' \
		--set global.imagePassword='$(shell lpassd show "Shared-RabbitMQ for Kubernetes/pivnet-dev-registry-ci" --notes | jq -r .token)' \
		--set global.imageRegistry="dev.registry.pivotal.io" \
		--set images.operator.name="dev.registry.pivotal.io/p-rabbitmq-for-kubernetes/rabbitmq-for-kubernetes-operator" \
		--set images.operator.tag="latest"

deploy-local:
	@echo "\e[1;31mThis target is expecting a file operator/values-local.yaml. Have you created this file?\e[0m"
	touch operator/values-local.yaml
	helm -n default install "${DEPLOYMENT_NAME}" operator/ -f operator/values-local.yaml

template:  ## helm template the operator chart
	@helm template "${CHART_NAME}"

values:  ## show helm operator chart values
	@helm show values "${CHART_NAME}"

# TODO: add make targets for ksm
