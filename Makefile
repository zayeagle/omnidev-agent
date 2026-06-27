# ── omnidev-agent Makefile ──────────────────────────────────────────────────

SHELL   := /bin/bash
GO      := /usr/local/go/bin/go
BIN_DIR := bin
SRC     := ./cmd/omnidev-agent
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.version=$(VERSION)

# Install destination
PREFIX         ?= $(HOME)/.local
INSTALL_BIN_DIR := $(PREFIX)/bin

# ── Cross-compile matrix ───────────────────────────────────────────────────
GOOSES   := linux darwin windows
GOARCHES := amd64 arm64
EXT_windows := .exe

.PHONY: all build run test vet clean install uninstall deploy help config
.PHONY: build-all fmt lint
.PHONY: $(foreach os,$(GOOSES),$(foreach arch,$(GOARCHES),build-$(os)-$(arch)))

# ── Default ─────────────────────────────────────────────────────────────────
all: vet build test

# ── Build (native) ─────────────────────────────────────────────────────────
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/omnidev-agent $(SRC)
	@echo "✔ Built: $(BIN_DIR)/omnidev-agent ($(VERSION))"

# ── Cross-compile: single target ───────────────────────────────────────────
# Usage: make build-linux-amd64, make build-darwin-arm64, make build-windows-amd64, etc.
define build_rule
build-$(1)-$(2):
	@mkdir -p $(BIN_DIR)
	GOOS=$(1) GOARCH=$(2) $(GO) build -ldflags "$(LDFLAGS)" \
		-o $(BIN_DIR)/omnidev-agent-$(1)-$(2)$$(EXT_$(1)) $(SRC)
	@echo "✔ Built: $(BIN_DIR)/omnidev-agent-$(1)-$(2)$$(EXT_$(1)) ($(VERSION))"
endef
$(foreach os,$(GOOSES),$(foreach arch,$(GOARCHES),$(eval $(call build_rule,$(os),$(arch)))))

# ── Cross-compile: all platforms ───────────────────────────────────────────
build-all:
	@$(MAKE) build-linux-amd64
	@$(MAKE) build-linux-arm64
	@$(MAKE) build-darwin-amd64
	@$(MAKE) build-darwin-arm64
	@$(MAKE) build-windows-amd64
	@$(MAKE) build-windows-arm64
	@echo ""
	@echo "── All platforms ──"
	@ls -lh $(BIN_DIR)/

# ── Code quality ───────────────────────────────────────────────────────────
fmt:
	$(GO) fmt ./...

lint: fmt vet
	$(GO) vet ./... 2>&1 | grep -v '^#' || true

# ── Run ────────────────────────────────────────────────────────────────────
run: build
	./$(BIN_DIR)/omnidev-agent

# ── Test ───────────────────────────────────────────────────────────────────
test:
	$(GO) test -v -timeout 60s ./tests/...

# ── Vet ────────────────────────────────────────────────────────────────────
vet:
	$(GO) vet ./...

# ── Clean ──────────────────────────────────────────────────────────────────
clean:
	rm -rf $(BIN_DIR)/
	@echo "✔ Cleaned build artifacts"

# ── Install ────────────────────────────────────────────────────────────────
install: build
	@install -d $(INSTALL_BIN_DIR)
	install -m 755 $(BIN_DIR)/omnidev-agent $(INSTALL_BIN_DIR)/omnidev-agent
	@echo "✔ Installed to $(INSTALL_BIN_DIR)/omnidev-agent"

# ── Uninstall ──────────────────────────────────────────────────────────────
uninstall:
	@rm -f $(INSTALL_BIN_DIR)/omnidev-agent
	@echo "✔ Removed $(INSTALL_BIN_DIR)/omnidev-agent"
	@if [ -f .omnidev-agent.json ]; then \
		rm -f .omnidev-agent.json; \
		echo "✔ Removed .omnidev-agent.json"; \
	fi
	@if [ -d $(HOME)/.config/omnidev-agent ]; then \
		echo "  ⚠  Global config still at ~/.config/omnidev-agent/"; \
		echo "     Run: rm -rf ~/.config/omnidev-agent  to fully remove"; \
	fi

# ── Config ─────────────────────────────────────────────────────────────────
config:
	@if [ -f .omnidev-agent.json ]; then \
		echo "⚠  .omnidev-agent.json already exists (skipped)"; \
	else \
		cp .omnidev-agent.json.sample .omnidev-agent.json; \
		echo "✔ Created .omnidev-agent.json from sample"; \
	fi
	@echo "  ✎ Edit .omnidev-agent.json to set your API key and provider."

# ── Deploy ─────────────────────────────────────────────────────────────────
deploy: install config
	@echo ""
	@echo "── Deployment complete ──"
	@echo "  Binary:   $(INSTALL_BIN_DIR)/omnidev-agent"
	@echo "  Config:   ./.omnidev-agent.json"
	@echo ""
	@if echo "$$PATH" | tr ':' '\n' | grep -qFx "$(INSTALL_BIN_DIR)"; then \
		echo "  ✔ $(INSTALL_BIN_DIR) is in PATH"; \
	else \
		echo "  ⚠  $(INSTALL_BIN_DIR) is NOT in PATH"; \
		echo "     Add to your shell profile:"; \
		echo "       export PATH=\"$(INSTALL_BIN_DIR):\$$PATH\""; \
	fi
	@echo ""

# ── Help ───────────────────────────────────────────────────────────────────
help:
	@echo "omnidev-agent — OmniDev AI Agent build system"
	@echo ""
	@echo "  make                        vet + build + test"
	@echo "  make build                  编译当前平台二进制"
	@echo "  make build-all              交叉编译全部 6 个平台"
	@echo "  make build-linux-amd64      单平台编译 (linux|darwin|windows × amd64|arm64)"
	@echo "  make run                    编译并运行"
	@echo "  make test                   运行测试"
	@echo "  make vet                    静态检查"
	@echo "  make fmt                    代码格式化"
	@echo "  make lint                   fmt + vet"
	@echo "  make clean                  清理 bin/"
	@echo "  make install                安装到 $(INSTALL_BIN_DIR)"
	@echo "  make uninstall              卸载 + 清理配置"
	@echo "  make deploy                 install + config + PATH 检查"
	@echo "  make help                   此帮助"
	@echo ""
	@echo "  Install path:  make install PREFIX=/usr/local"
	@echo "  Output names:  bin/omnidev-agent-linux-amd64, bin/omnidev-agent-windows-amd64.exe, ..."
