# Comandos del proyecto. Los targets del stack completo (up, test, …) se agregan en la etapa de implementación.
# Requiere GNU Make y Docker. En Windows: Git Bash o WSL.

SHELL := /bin/bash

MOCK_COMPOSE := docker compose -f deploy/docker-compose.mock.yml
SPECTRAL_IMAGE := stoplight/spectral:6.14.3
CONTRACT := docs/contracts/benefits-api.v1.yaml
RULESET := docs/contracts/.spectral.yaml

.PHONY: help mock-up mock-down mock-logs contract-lint

help: ## Lista los comandos disponibles
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "} {printf "  %-15s %s\n", $$1, $$2}'

mock-up: ## Levanta el mock del contrato de partners en http://localhost:4010
	$(MOCK_COMPOSE) up -d

mock-down: ## Baja el mock
	$(MOCK_COMPOSE) down

mock-logs: ## Muestra los logs del mock (requests y validaciones)
	$(MOCK_COMPOSE) logs -f benefits-mock

contract-lint: ## Valida el contrato con Spectral
	docker run --rm -v "$(CURDIR):/work" -w /work $(SPECTRAL_IMAGE) \
		lint $(CONTRACT) --ruleset $(RULESET) --fail-severity=warn
