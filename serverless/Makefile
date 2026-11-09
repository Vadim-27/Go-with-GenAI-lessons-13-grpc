# LibraTrack · Lambda demo (Урок 11). `make help` — список цілей.
SHELL := /usr/bin/env bash

.DEFAULT_GOAL := help
.PHONY: help test build up up-ephemeral down deploy-cli deploy-tf plan-tf smoke bench cleanup clean sam-build sam-invoke sam-api

help: ## Показати цілі
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

test: ## Юніт-тести (без Docker і без Floci)
	@test -f go.sum || go mod tidy
	go vet ./...
	go test -count=1 -cover ./...

build: ## Зібрати і запакувати Lambda (build/*.zip)
	scripts/build.sh

up: ## Запустити Floci (docker compose) і завантажити образ Lambda
	scripts/up.sh

up-ephemeral: ## Запустити Floci у режимі «кожен виклик = холодний старт»
	scripts/up.sh --ephemeral

down: ## Зупинити Floci
	docker compose down

deploy-cli: build ## Розгорнути у Floci через AWS CLI
	scripts/deploy-cli.sh

plan-tf: build ## Terraform: лише init/validate/plan (читаємо diff!)
	scripts/deploy-tf.sh plan

deploy-tf: build ## Terraform: apply (після того, як прочитали plan)
	scripts/deploy-tf.sh apply

smoke: ## Smoke-тест на Floci (чекліст зі слайда 45)
	scripts/test.sh

bench: ## Порівняти cold та warm виклики
	scripts/bench-cold-start.sh 10

cleanup: ## Видалити ресурси демо з Floci
	scripts/cleanup.sh

clean: ## Видалити build/ і локальні артефакти
	rm -rf build .api-id .aws-sam

# ── AWS SAM CLI (слайд 20). Не потребує Floci — лише Docker. ────────────────
# SAM викликає ці цілі як `make build-<LogicalId>`.
build-WebhookFunction:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -ldflags="-s -w" -o $(ARTIFACTS_DIR)/bootstrap ./cmd/webhook
build-ThumbnailFunction:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -ldflags="-s -w" -o $(ARTIFACTS_DIR)/bootstrap ./cmd/thumbnail
build-FineFunction:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -ldflags="-s -w" -o $(ARTIFACTS_DIR)/bootstrap ./cmd/fine

sam-build: ## sam build
	sam build -t sam/template.yaml
sam-invoke: sam-build ## sam local invoke на events/valid.json та invalid.json
	sam local invoke WebhookFunction -e events/valid.json
	sam local invoke WebhookFunction -e events/invalid.json
	sam local invoke FineFunction -e events/fine_valid.json
	sam local invoke FineFunction -e events/fine_invalid.json
sam-api: sam-build ## sam local start-api → http://127.0.0.1:3000
	sam local start-api
