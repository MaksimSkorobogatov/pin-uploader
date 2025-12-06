CLIENT_CONFIG ?= $(HOME)/.config/pin-uploader/config.yaml

.PHONY: help
help: ## Show the help prompt.
	grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build all binaries
	go build -o bin/ ./...

.PHONY: fmt
fmt: ## Run gofmt on project sources
	gofmt -w .

.PHONY: configure
configure: ## Create default client config (~/.config/pin-uploader/config.yaml); set the correct LLM API token on the server
	@mkdir -p $(dir $(CLIENT_CONFIG))
	@printf '%s\n' "# pin-uploader client config" \
		'server_address: "http://localhost:8080"' \
		'encryption_key: "BASE64_32_BYTE_KEY"' \
		"# Ensure your server is configured with the correct LLM API token." > $(CLIENT_CONFIG)
	@echo "Wrote $(CLIENT_CONFIG). Update encryption_key/server_address and ensure the server LLM API token is set."

install: build ## Build and install the server to linux system
	sh install.sh
