# Makefile для микросервиса заказов с Kafka и PostgreSQL

# Переменные
BIN_DIR = bin
SERVER_BIN = $(BIN_DIR)/server
PRODUCER_BIN = $(BIN_DIR)/producer
DOCKER_COMPOSE = docker-compose
GO = go
GO_BUILD = CGO_ENABLED=0 GOOS=linux $(GO) build
GO_TEST = $(GO) test -v
GO_MOD = $(GO) mod

# Цели по умолчанию
.PHONY: all
all: build

# Создание директории для бинарных файлов
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

# Сборка сервера
.PHONY: build-server
build-server: $(BIN_DIR)
	$(GO_BUILD) -o $(SERVER_BIN) ./cmd/server

# Сборка продюсера
.PHONY: build-producer
build-producer: $(BIN_DIR)
	$(GO_BUILD) -o $(PRODUCER_BIN) ./cmd/producer

# Сборка всего
.PHONY: build
build: build-server build-producer

# Запуск сервера локально
.PHONY: run-server
run-server:
	$(GO) run ./cmd/server

# Запуск продюсера локально
.PHONY: run-producer
run-producer:
	$(GO) run ./cmd/producer

# Запуск всех компонентов через Docker Compose
.PHONY: docker-up
docker-up:
	$(DOCKER_COMPOSE) up -d

# Остановка всех компонентов Docker Compose
.PHONY: docker-down
docker-down:
	$(DOCKER_COMPOSE) down

# Перезапуск Docker Compose
.PHONY: docker-restart
docker-restart: docker-down docker-up

# Просмотр логов
.PHONY: docker-logs
docker-logs:
	$(DOCKER_COMPOSE) logs -f

# Просмотр логов приложения
.PHONY: docker-logs-app
docker-logs-app:
	$(DOCKER_COMPOSE) logs -f app

# Просмотр логов продюсера
.PHONY: docker-logs-producer
docker-logs-producer:
	$(DOCKER_COMPOSE) logs -f producer

# Очистка бинарных файлов
.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

# Полная очистка (включая Docker)
.PHONY: clean-all
clean-all: clean docker-down
	$(DOCKER_COMPOSE) down -v

# Показать статус Docker контейнеров
.PHONY: docker-status
docker-status:
	$(DOCKER_COMPOSE) ps

# Пересобрать и перезапустить приложение
.PHONY: docker-rebuild-app
docker-rebuild-app:
	$(DOCKER_COMPOSE) up -d --build app

# Пересобрать и перезапустить продюсер
.PHONY: docker-rebuild-producer
docker-rebuild-producer:
	$(DOCKER_COMPOSE) up -d --build producer

# Зайти в контейнер с базой данных
.PHONY: db-shell
db-shell:
	$(DOCKER_COMPOSE) exec postgres psql -U user -d orders_db

# Зайти в контейнер с приложением
.PHONY: app-shell
app-shell:
	$(DOCKER_COMPOSE) exec app sh

# Проверка зависимостей
.PHONY: deps
deps:
	$(GO_MOD) download
	$(GO_MOD) tidy

# Помощь (список всех команд)
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