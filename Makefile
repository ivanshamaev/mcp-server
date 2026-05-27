.PHONY: build test lint clean inspect test-mcp run tidy \
        release snapshot release-dry-run tag-patch tag-minor tag-major changelog

BINARY := bin/mcp-server
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

# ──────────────────────────────────────────
# Сборка
# ──────────────────────────────────────────

build: tidy
	CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/server
	@echo "✅ Built: $(BINARY)"

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-linux ./cmd/server

build-darwin:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BINARY)-darwin ./cmd/server

# ──────────────────────────────────────────
# Тесты
# ──────────────────────────────────────────

test:
	go test ./... -v -timeout 30s

test-cover:
	go test ./... -coverprofile=coverage.out -timeout 30s
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report: coverage.html"

test-race:
	go test ./... -race -timeout 60s

# ──────────────────────────────────────────
# Протокольный тест MCP (E2E)
# ──────────────────────────────────────────

test-mcp: build
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	{ \
		printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"make-test","version":"1.0"}}}\n'; \
		sleep 0.1; \
		printf '{"jsonrpc":"2.0","method":"notifications/initialized"}\n'; \
		sleep 0.1; \
		printf '{"jsonrpc":"2.0","id":2,"method":"tools/list"}\n'; \
		sleep 0.2; \
		printf '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"metrika_get_counters","arguments":{}}}\n'; \
		sleep 2; \
	} | ACCESS_TOKEN="$$ACCESS_TOKEN" ./$(BINARY) | python3 -m json.tool --no-ensure-ascii 2>/dev/null || cat

# ──────────────────────────────────────────
# Отладка
# ──────────────────────────────────────────

inspect: build
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	ACCESS_TOKEN="$$ACCESS_TOKEN" npx @modelcontextprotocol/inspector ./$(BINARY)

run: build
	@if [ -f .env ]; then export $$(grep -v '^#' .env | xargs); fi; \
	ACCESS_TOKEN="$$ACCESS_TOKEN" LOG_LEVEL=debug ./$(BINARY)

# ──────────────────────────────────────────
# Качество кода
# ──────────────────────────────────────────

lint:
	go vet ./...
	@if command -v golangci-lint &>/dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "⚠️  golangci-lint not installed, running go vet only"; \
	fi

fmt:
	gofmt -w .
	@if command -v goimports &>/dev/null; then goimports -w .; fi

tidy:
	go mod tidy

# ──────────────────────────────────────────
# Утилиты
# ──────────────────────────────────────────

clean:
	rm -rf bin/ coverage.out coverage.html

install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "✅ Tools installed"

# Показать размер бинарника
size: build
	@ls -lh $(BINARY)
	@file $(BINARY)

# ──────────────────────────────────────────
# Версионирование и релиз
# ──────────────────────────────────────────

# Текущая версия из последнего тега
CURRENT_VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

# Выпустить патч-версию: v1.2.3 → v1.2.4
tag-patch:
	$(eval NEXT := $(shell echo $(CURRENT_VERSION) | awk -F. '{printf "%s.%s.%d", $$1, $$2, $$3+1}'))
	@echo "Текущая версия: $(CURRENT_VERSION) → $(NEXT)"
	@read -p "Подтвердить тег $(NEXT)? [y/N] " ans && [ "$$ans" = "y" ]
	git tag -a $(NEXT) -m "Release $(NEXT)"
	git push origin $(NEXT)
	@echo "✅ Тег $(NEXT) создан и отправлен. GitHub Actions запустит релиз."

# Выпустить minor-версию: v1.2.3 → v1.3.0
tag-minor:
	$(eval NEXT := $(shell echo $(CURRENT_VERSION) | awk -F. '{printf "%s.%d.0", $$1, $$2+1}'))
	@echo "Текущая версия: $(CURRENT_VERSION) → $(NEXT)"
	@read -p "Подтвердить тег $(NEXT)? [y/N] " ans && [ "$$ans" = "y" ]
	git tag -a $(NEXT) -m "Release $(NEXT)"
	git push origin $(NEXT)
	@echo "✅ Тег $(NEXT) создан и отправлен. GitHub Actions запустит релиз."

# Выпустить major-версию: v1.2.3 → v2.0.0
tag-major:
	$(eval NEXT := $(shell echo $(CURRENT_VERSION) | awk -F. '{printf "v%d.0.0", $$1+1}' | sed 's/vv/v/'))
	@echo "Текущая версия: $(CURRENT_VERSION) → $(NEXT)"
	@read -p "⚠️  MAJOR релиз $(NEXT)! Подтвердить? [y/N] " ans && [ "$$ans" = "y" ]
	git tag -a $(NEXT) -m "Release $(NEXT)"
	git push origin $(NEXT)
	@echo "✅ Тег $(NEXT) создан и отправлен. GitHub Actions запустит релиз."

# Локальная сборка snapshot (как goreleaser, без публикации)
snapshot:
	@if ! command -v goreleaser &>/dev/null; then \
		echo "Установка goreleaser..."; \
		go install github.com/goreleaser/goreleaser/v2@latest; \
	fi
	goreleaser release --snapshot --clean
	@echo "✅ Snapshot собран в ./dist/"
	@ls -lh dist/yametrika-mcp_*/yametrika-mcp* 2>/dev/null || ls dist/

# Проверить .goreleaser.yml без сборки
release-dry-run:
	@if ! command -v goreleaser &>/dev/null; then \
		go install github.com/goreleaser/goreleaser/v2@latest; \
	fi
	goreleaser check
	goreleaser release --snapshot --skip=publish --clean
	@echo "✅ Dry run успешен. Проверь ./dist/"

# Установить goreleaser локально
install-goreleaser:
	go install github.com/goreleaser/goreleaser/v2@latest
	@echo "✅ goreleaser установлен: $$(goreleaser --version)"

# Показать текущую версию
version:
	@echo "Текущий тег: $(CURRENT_VERSION)"
	@echo "Полная версия: $$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
	@echo "Коммит: $$(git rev-parse --short HEAD 2>/dev/null)"
