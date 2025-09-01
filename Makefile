BIN_DIR = bin
SERVER_BIN = $(BIN_DIR)/server
PRODUCER_BIN = $(BIN_DIR)/producer
DOCKER_COMPOSE = docker-compose
GO = go
GO_BUILD = CGO_ENABLED=0 GOOS=linux $(GO) build
GO_TEST = $(GO) test -v
GO_MOD = $(GO) mod

.PHONY: all
all: build

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

.PHONY: build-server
build-server: $(BIN_DIR)
	$(GO_BUILD) -o $(SERVER_BIN) ./cmd/server

.PHONY: build-producer
build-producer: $(BIN_DIR)
	$(GO_BUILD) -o $(PRODUCER_BIN) ./cmd/producer

.PHONY: build
build: build-server build-producer

.PHONY: run-server
run-server:
	$(GO) run ./cmd/server

.PHONY: run-producer
run-producer:
	$(GO) run ./cmd/producer

.PHONY: docker-up
docker-up:
	$(DOCKER_COMPOSE) up -d

.PHONY: docker-down
docker-down:
	$(DOCKER_COMPOSE) down

.PHONY: docker-restart
docker-restart: docker-down docker-up

.PHONY: docker-logs
docker-logs:
	$(DOCKER_COMPOSE) logs -f

.PHONY: docker-logs-app
docker-logs-app:
	$(DOCKER_COMPOSE) logs -f app

.PHONY: docker-logs-producer
docker-logs-producer:
	$(DOCKER_COMPOSE) logs -f producer

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

.PHONY: clean-all
clean-all: clean docker-down
	$(DOCKER_COMPOSE) down -v

.PHONY: docker-status
docker-status:
	$(DOCKER_COMPOSE) ps

.PHONY: docker-rebuild-app
docker-rebuild-app:
	$(DOCKER_COMPOSE) up -d --build app

.PHONY: docker-rebuild-producer
docker-rebuild-producer:
	$(DOCKER_COMPOSE) up -d --build producer

.PHONY: db-shell
db-shell:
	$(DOCKER_COMPOSE) exec postgres psql -U user -d orders_db

.PHONY: app-shell
app-shell:
	$(DOCKER_COMPOSE) exec app sh

.PHONY: deps
deps:
	$(GO_MOD) download
	$(GO_MOD) tidy

.PHONY: help
help:
	@echo "Доступные команды:"
	@echo "  build               - Сборка сервера и продюсера"
	@echo "  build-server        - Сборка только сервера"
	@echo "  build-producer      - Сборка только продюсера"
	@echo "  run-server          - Запуск сервера локально"
	@echo "  run-producer        - Запуск продюсера локально"
	@echo "  docker-up           - Запуск всех сервисов через Docker"
	@echo "  docker-down         - Остановка всех сервисов Docker"
	@echo "  docker-restart      - Перезапуск Docker сервисов"
	@echo "  docker-logs         - Просмотр логов всех сервисов"
	@echo "  docker-logs-app     - Просмотр логов приложения"
	@echo "  docker-logs-producer - Просмотр логов продюсера"
	@echo "  clean               - Очистка бинарных файлов"
	@echo "  clean-all           - Полная очистка (включая Docker тома)"
	@echo "  docker-status       - Показать статус Docker контейнеров"
	@echo "  docker-rebuild-app  - Пересобрать и перезапустить приложение"
	@echo "  docker-rebuild-producer - Пересобрать и перезапустить продюсер"
	@echo "  db-shell            - Зайти в консоль PostgreSQL"
	@echo "  app-shell           - Зайти в контейнер приложения"
	@echo "  deps                - Обновление зависимостей"
	@echo "  help                - Показать эту справку"