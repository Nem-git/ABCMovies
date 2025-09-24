GO := go
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean

# Directories
BUILD_DIR := build
PLUGIN_DIR := plugins
CMD_DIR := cmd/abcmovies

BINARY_NAME=abcmovies.out

all: build build-plugins

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

build:
	@mkdir -p $(BUILD_DIR)
	@echo "Building main.go..."
	$(GOBUILD) -o $(BUILD_DIR) $(CMD_DIR)/main.go

build-plugins:
	@mkdir -p $(BUILD_DIR)/plugins
	@echo "Building plugins..."
	@for plugin in $(PLUGIN_DIR)/*/ ; do \
		echo $$plugin && \
		if [ -f $$plugin/go.mod ]; then \
			plugin_name=$$(basename $$plugin); \
			echo "Building plugin: $$plugin_name"; \
			cd $$plugin && go mod tidy && \
			$(GOBUILD) -buildmode=plugin -o ../../$(BUILD_DIR)/plugins/$$plugin_name.so || exit 1; \
			cd ../../; \
		fi \
	done