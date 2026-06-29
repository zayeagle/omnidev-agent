# ── omnidev-agent Makefile ──────────────────────────────────────────────────

SHELL   := /bin/bash
GO      ?= go
BIN_DIR := bin
SRC     := ./cmd/omnidev-agent
VERSION := $(shell tr -d '\r\n ' < VERSION 2>/dev/null || echo "0.0.0")

# BUILD_TIME is expanded in the recipe so each build/deploy gets a fresh timestamp.
GO_LDFLAGS = -ldflags "-X main.appVersion=$(VERSION) -X 'main.buildTime=$$(date '+%Y-%m-%d %H:%M:%S')'"

# Install destination
PREFIX         ?= $(HOME)/.local
INSTALL_BIN_DIR := $(PREFIX)/bin

# Global config (install once, use from any directory)
GLOBAL_CONFIG_DIR  := $(HOME)/.omnidev-agent
GLOBAL_CONFIG      := $(GLOBAL_CONFIG_DIR)/config.json
PROJECT_CONFIG     := $(CURDIR)/.omnidev-agent.json
BUILD_BINARY       := $(CURDIR)/$(BIN_DIR)/omnidev-agent
INSTALL_BINARY     := $(INSTALL_BIN_DIR)/omnidev-agent

# ── Cross-compile matrix ───────────────────────────────────────────────────
GOOSES   := linux darwin windows
GOARCHES := amd64 arm64
EXT_windows := .exe

.PHONY: all build rebuild run test vet clean install install-binary uninstall deploy help config config-local
.PHONY: build-all fmt lint bump-patch bump-minor bump-major publish
.PHONY: $(foreach os,$(GOOSES),$(foreach arch,$(GOARCHES),build-$(os)-$(arch)))

# ── Default ─────────────────────────────────────────────────────────────────
all: vet build test

# ── Build (native) ─────────────────────────────────────────────────────────
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GO_LDFLAGS) -o $(BIN_DIR)/omnidev-agent $(SRC)
	@echo "✔ Built: $(BIN_DIR)/omnidev-agent (v$(VERSION))"

# Force a full rebuild (used by deploy so installed binary always matches sources).
rebuild:
	@mkdir -p $(BIN_DIR)
	@rm -f $(BIN_DIR)/omnidev-agent
	$(GO) build -a $(GO_LDFLAGS) -o $(BIN_DIR)/omnidev-agent $(SRC)
	@echo "✔ Rebuilt: $(BIN_DIR)/omnidev-agent (v$(VERSION))"

# ── Version bump (semver in VERSION file) ────────────────────────────────────
bump-patch:
	@bash scripts/bump-version.sh patch

bump-minor:
	@bash scripts/bump-version.sh minor

bump-major:
	@bash scripts/bump-version.sh major

# Bump patch, commit VERSION, and push to origin (use instead of raw git push).
publish: bump-patch
	@NEW=$$(tr -d '\r\n ' < VERSION); \
	git add VERSION; \
	git commit -m "chore: release v$$NEW"; \
	git push origin main; \
	echo "✔ Published v$$NEW"

# ── Cross-compile: single target ───────────────────────────────────────────
# Usage: make build-linux-amd64, make build-darwin-arm64, make build-windows-amd64, etc.
define build_rule
build-$(1)-$(2):
	@mkdir -p $(BIN_DIR)
	GOOS=$(1) GOARCH=$(2) $(GO) build $(GO_LDFLAGS) \
		-o $(BIN_DIR)/omnidev-agent-$(1)-$(2)$$(EXT_$(1)) $(SRC)
	@echo "✔ Built: $(BIN_DIR)/omnidev-agent-$(1)-$(2)$$(EXT_$(1)) (v$(VERSION))"
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
install-binary:
	@test -f $(BIN_DIR)/omnidev-agent || (echo "✗ missing $(BIN_DIR)/omnidev-agent — run make build first" && exit 1)
	@install -d $(INSTALL_BIN_DIR)
	install -m 755 $(BIN_DIR)/omnidev-agent $(INSTALL_BIN_DIR)/omnidev-agent
	@echo "✔ Installed to $(INSTALL_BIN_DIR)/omnidev-agent (v$(VERSION))"

install: build install-binary

# ── Uninstall ──────────────────────────────────────────────────────────────
uninstall:
	@rm -f $(INSTALL_BIN_DIR)/omnidev-agent
	@echo "✔ Removed $(INSTALL_BIN_DIR)/omnidev-agent"
	@if [ -f $(GLOBAL_CONFIG) ]; then \
		echo "  ⚠  Global config kept at $(GLOBAL_CONFIG)"; \
		echo "     Run: rm -f $(GLOBAL_CONFIG)  to remove"; \
	fi

# ── Config ─────────────────────────────────────────────────────────────────
# Global config for deploy / install-once-use-everywhere (any cwd).
config:
	@install -d $(GLOBAL_CONFIG_DIR)
	@if [ -f $(GLOBAL_CONFIG) ]; then \
		echo "⚠  $(GLOBAL_CONFIG) already exists (skipped)"; \
	else \
		cp .omnidev-agent.json.sample $(GLOBAL_CONFIG); \
		chmod 600 $(GLOBAL_CONFIG); \
		echo "✔ Created $(GLOBAL_CONFIG) from sample"; \
	fi
	@echo "  ✎ Edit $(GLOBAL_CONFIG) to set your API key and provider."

# Optional per-project override (higher priority than global when cwd has this file).
config-local:
	@if [ -f .omnidev-agent.json ]; then \
		echo "⚠  .omnidev-agent.json already exists (skipped)"; \
	else \
		cp .omnidev-agent.json.sample .omnidev-agent.json; \
		echo "✔ Created .omnidev-agent.json from sample"; \
	fi
	@echo "  ✎ Edit .omnidev-agent.json (optional project override)."

# ── Deploy ─────────────────────────────────────────────────────────────────
deploy: rebuild install-binary config config-local
	@echo ""
	@echo "══════════════════════════════════════════════════════════════"
	@echo "  Deployment complete"
	@echo "══════════════════════════════════════════════════════════════"
	@echo ""
	@echo "  [Binary]"
	@echo "    Build artifact:   $(BUILD_BINARY)"
	@echo "    Installed binary: $(INSTALL_BINARY)"
	@echo ""
	@echo "  [Configuration]"
	@echo "    Global config:  $(GLOBAL_CONFIG)"
	@echo "    Project config: $(PROJECT_CONFIG)"
	@echo ""
	@echo "  [Config priority — higher wins]"
	@echo "    1. CLI flags / environment variables (e.g. OMNIDEV_API_KEY)"
	@echo "    2. Project config: $(PROJECT_CONFIG)"
	@echo "       (only when current working directory is the project root)"
	@echo "    3. Global config:  $(GLOBAL_CONFIG)"
	@echo "       (used from any directory when project file is absent or not loaded)"
	@echo "    4. Built-in defaults"
	@echo ""
	@echo "  Tip: edit global config for install-once-use-everywhere."
	@echo "       edit project config only when this project needs different settings."
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
	@echo "  make install                编译并安装到 $(INSTALL_BIN_DIR)"
	@echo "  make uninstall              卸载 + 清理配置"
	@echo "  make config                 初始化全局配置 (~/.omnidev-agent/config.json)"
	@echo "  make config-local           初始化项目配置 (./.omnidev-agent.json，可选)"
	@echo "  make deploy                 强制全量重编译 + 安装 + 全局/项目 config"
	@echo "  make rebuild                强制全量重编译 (go build -a)"
	@echo "  make bump-patch             0.0.0 -> 0.0.1 (VERSION file)"
	@echo "  make publish                bump-patch + commit VERSION + push"
	@echo "  make help                   此帮助"
	@echo ""
	@echo "  Install path:  make install PREFIX=/usr/local"
	@echo "  Output names:  bin/omnidev-agent-linux-amd64, bin/omnidev-agent-windows-amd64.exe, ..."
