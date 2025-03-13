APP_NAME = homework
BUILD_DIR = build
SRC_DIR = ./cmd/main.go
GO_FILES = $(shell find . -type f -name '*.go')

ifeq ($(POSTGRES_SETUP_TEST),)
	POSTGRES_SETUP_TEST := user=test password=test dbname=test host=localhost port=5432 sslmode=disable
endif

MIGRATION_FOLDER=$(CURDIR)/migrations

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

.PHONY: migration-create
migration-create:
	goose -dir "$(MIGRATION_FOLDER)" create "$(name)" sql

.PHONY: migration-up
migration-up:
	goose -dir "$(MIGRATION_FOLDER)" postgres "$(POSTGRES_SETUP_TEST)" up

.PHONY: migration-down
migration-down:
	goose -dir "$(MIGRATION_FOLDER)" postgres "$(POSTGRES_SETUP_TEST)" down

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)