APP_NAME = homework
BUILD_DIR = build
SRC_DIR = ./cmd/main.go
GO_FILES = $(shell find . -type f -name '*.go')

ifeq ($(POSTGRES_SETUP_TEST),)
	POSTGRES_SETUP_TEST := user=test password=test dbname=test host=localhost port=5432 sslmode=disable
endif

MIGRATION_FOLDER=$(CURDIR)/migrations

# Docker compose file for test environment
DOCKER_COMPOSE_FILE = docker-compose.yml

.PHONY: build
build: lint
	mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) $(SRC_DIR)

.PHONY: run
run: build
	./$(BUILD_DIR)/$(APP_NAME)

.PHONY: install-linters
install-linters:
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
	go install github.com/uudashr/gocognit/cmd/gocognit@latest

.PHONY: lint
lint:
	@echo "Running gocyclo..."
	@gocyclo -over 510 .
	@echo "Running gocognit..."
	@gocognit -over 510 $(GO_FILES)

# Database migration commands
.PHONY: migration-create
migration-create:
	goose -dir "$(MIGRATION_FOLDER)" create "$(name)" sql

.PHONY: migration-up
migration-up:
	goose -dir "$(MIGRATION_FOLDER)" postgres "$(POSTGRES_SETUP_TEST)" up

.PHONY: migration-down
migration-down:
	goose -dir "$(MIGRATION_FOLDER)" postgres "$(POSTGRES_SETUP_TEST)" down

# Test environment commands
.PHONY: test-env-up
test-env-up:
	@echo "Starting test environment..."
	docker-compose -f $(DOCKER_COMPOSE_FILE) up -d
	@echo "Waiting for services to be ready..."
	sleep 5
	@echo "Test environment is up and running"

.PHONY: test-env-down
test-env-down:
	@echo "Stopping test environment..."
	docker-compose -f $(DOCKER_COMPOSE_FILE) down
	@echo "Test environment stopped"

# Test commands
.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	go test -v ./... -tags=unit
	@echo "Unit tests completed"

.PHONY: test-integration
test-integration: test-env-up migration-up
	@echo "Running integration tests..."
	go test -v ./... -tags=integration
	@echo "Integration tests completed"

.PHONY: test-all
test-all: test-unit test-integration
	@echo "All tests completed"

# Database cleanup command
.PHONY: db-clean
db-clean:
	@echo "Cleaning test database..."
	goose -dir "$(MIGRATION_FOLDER)" postgres "$(POSTGRES_SETUP_TEST)" down-to 0
	@echo "Test database cleaned"

# Mock generation commands
.PHONY: generate-mocks clean-mocks
generate-mocks:
	@echo "Generating mocks using go:generate..."
	go generate ./...
	@echo "Mocks generated successfully!"

clean-mocks:
	@echo "Cleaning up mock files..."
	find . -type f -name "*_mock.go" -delete
	@echo "Mocks cleaned!"

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)