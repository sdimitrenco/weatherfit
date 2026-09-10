BINARY           := weatherbot
LOCAL_BIN        := $(CURDIR)/bin
GOLANGCI_VERSION := v2.13.2
GOLANGCI         := $(LOCAL_BIN)/golangci-lint
DOCKER_IMAGE     := weatherbot:local

.PHONY: help run build test test-race test-race-docker cover lint fmt tidy docker docker-up docker-down clean

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

run: ## запустить бота локально, подхватив .env
	@test -f .env || { echo "нет .env, скопируй .env.example"; exit 1; }
	set -a; . ./.env; set +a; go run ./cmd/weatherbot

build: ## собрать бинарник в bin/
	@mkdir -p $(LOCAL_BIN)
	go build -trimpath -ldflags "-s -w" -o $(LOCAL_BIN)/$(BINARY) ./cmd/weatherbot

test: ## прогнать тесты
	go test ./...

test-race: ## прогнать тесты с детектором гонок (нужен gcc)
	CGO_ENABLED=1 go test -race ./...

test-race-docker: ## прогнать тесты с детектором гонок в контейнере, если gcc нет
	docker run --rm -v "$(CURDIR)":/src -w /src -e GOCACHE=/tmp/gocache -e GOPATH=/tmp/gopath \
		golang:1.26-alpine sh -c 'apk add --no-cache gcc musl-dev >/dev/null && go test -race ./...'

cover: ## посчитать покрытие тестами
	go test -coverprofile=coverage.out ./... >/dev/null
	go tool cover -func=coverage.out | tail -1

lint: $(GOLANGCI) ## прогнать golangci-lint
	$(GOLANGCI) run

$(GOLANGCI):
	@mkdir -p $(LOCAL_BIN)
	GOBIN=$(LOCAL_BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION)

fmt: ## отформатировать код
	gofmt -w ./cmd ./internal

tidy: ## привести go.mod и go.sum в порядок
	go mod tidy

docker: ## собрать docker-образ
	docker build -t $(DOCKER_IMAGE) .

docker-up: ## поднять контейнер через docker compose
	docker compose up -d --build

docker-down: ## остановить контейнер
	docker compose down

clean: ## удалить артефакты сборки
	rm -rf $(LOCAL_BIN)
