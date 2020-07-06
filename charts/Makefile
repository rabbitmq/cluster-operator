# runs the target list by default
.DEFAULT_GOAL = list
.PHONY: list

# Insert a comment starting with '##' after a target, and it will be printed by 'make' and 'make list'
list:
	@echo "The most used targets: \n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# helm setup/configure
DEPLOYMENT_NAME=rabbitmq-operator

setup: ## register the dev pivnet registry with helm
	@echo "Registering chart repo"
	@helm repo add "$$CHARTREPO_NAME" \
		"$$HELM_REPO" \
		--username="$$HELM_REPO_USERNAME" \
		--password="$$HELM_REPO_PASSWORD"

CHART_NAME=${CHARTREPO_NAME}/rabbitmq-operator
destroy: ## helm uninstall operator chart
	helm -n default uninstall "${DEPLOYMENT_NAME}"

deploy: ## helm install latest version of operator chart hosted in Dev PivNet
	helm -n default install "${DEPLOYMENT_NAME}" "${CHART_NAME}" \
		--set global.imageUsername="$$DOCKER_REGISTRY_USERNAME" \
		--set global.imagePassword="$$DOCKER_REGISTRY_PASSWORD" \
		--set global.imageRegistry="$$DOCKER_REGISTRY_SERVER" \
		--set images.operator.name="$$DOCKER_REGISTRY_SERVER"/"$$OPERATOR_IMAGE" \
		--set images.operator.tag="latest"

deploy-local:
	@echo "\e[1;31mThis target is expecting a file operator/values-local.yaml. Have you created this file?\e[0m"
	touch operator/values-local.yaml
	helm -n default install "${DEPLOYMENT_NAME}" operator/ -f operator/values-local.yaml

template:  ## helm template the operator chart
	@helm template "${CHART_NAME}"

values:  ## show helm operator chart values
	@helm show values "${CHART_NAME}"
