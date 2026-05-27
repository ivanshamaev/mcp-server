---
layout: default
title: CI/CD и релизы
nav_order: 10
permalink: /cicd
---

# CI/CD и релизы
{: .no_toc }

<details open markdown="block">
  <summary>Содержание</summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Обзор пайплайна

```
push main / PR          push v*.*.*          изменение docs/
      │                       │                      │
      ▼                       ▼                      ▼
  ci.yml                release.yml             docs.yml
      │                       │                      │
  ┌───┴────┐            go test -race          jekyll build
  │        │                  │                      │
 test    build           goreleaser            deploy Pages
 lint    check           release --clean
```

---

## CI Workflow (`.github/workflows/ci.yml`)

```yaml
name: CI

on:
  push:
    branches: [main]
    paths-ignore: ["**.md", "docs/**", ".gitignore"]
  pull_request:
    branches: [main]
    paths-ignore: ["**.md", "docs/**", ".gitignore"]

env:
  # Убирает warning о Node.js 20 deprecation
  FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true

permissions:
  contents: read
```

### Job: test — матрица платформ

```yaml
jobs:
  test:
    name: Test (${{ matrix.os }}, Go ${{ matrix.go }})
    runs-on: ${{ matrix.os }}
    strategy:
      matrix:
        include:
          - { os: ubuntu-latest, go: "1.22" }   # основная конфигурация
          - { os: ubuntu-latest, go: "1.23" }   # новая версия Go
          - { os: macos-latest,  go: "1.22" }   # нативный macOS тест

    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go }}
          cache: true

      - run: go mod download && go mod verify
      - run: go vet ./...
      - run: go test ./... -v -race -timeout 60s
      - run: |
          go test ./... -coverprofile=coverage.out -covermode=atomic
          go tool cover -func=coverage.out | tail -1
```

{: .note }
> **macOS runner**: Кросс-компиляция darwin-бинарника с Ubuntu не гарантирует корректную работу на реальном Mac (отличия в syscall, stdio поведении). `macos-latest` runner запускает нативный Go код.

### Job: build — кросс-компиляция

```yaml
  build:
    name: Build (${{ matrix.goos }}/${{ matrix.goarch }})
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - { goos: linux,   goarch: amd64 }
          - { goos: linux,   goarch: arm64 }
          - { goos: darwin,  goarch: amd64 }
          - { goos: darwin,  goarch: arm64 }
          - { goos: windows, goarch: amd64 }

    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
          cache: true
      - name: Build
        env:
          GOOS: ${{ matrix.goos }}
          GOARCH: ${{ matrix.goarch }}
          CGO_ENABLED: "0"       # статическая линковка, нет зависимости от libc
        run: |
          go build \
            -ldflags="-s -w -X main.version=ci-$(git rev-parse --short HEAD)" \
            -o bin/mcp-server-${{ matrix.goos }}-${{ matrix.goarch }} \
            ./cmd/server
```

### Job: lint — golangci-lint

```yaml
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
          cache: true
      - uses: golangci/golangci-lint-action@v6
        with:
          version: v1.59
          args: --timeout=5m
```

### Job: goreleaser-check

```yaml
  goreleaser-check:
    name: Goreleaser snapshot
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0      # нужна полная история для goreleaser
      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
          cache: true
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --snapshot --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## Release Workflow (`.github/workflows/release.yml`)

```yaml
name: Release

on:
  push:
    tags: ["v*.*.*"]
  workflow_dispatch:     # ручной запуск из GitHub UI
    inputs:
      tag:
        description: "Tag to release (e.g. v1.0.0)"
        required: true

env:
  FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true

permissions:
  contents: write        # создание GitHub Release + загрузка артефактов

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: "1.22"
          cache: true

      - name: Run tests
        run: go test ./... -race

      - name: Release
        uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

---

## Goreleaser (`.goreleaser.yml`)

{% raw %}
```yaml
version: 2

project_name: my-api-mcp

before:
  hooks:
    - go mod tidy
    - go mod verify
    - go vet ./...

builds:
  - id: my-api-mcp
    main: ./cmd/server
    binary: my-api-mcp
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w
      - -X main.version={{ .Version }}
      - -X main.commit={{ .ShortCommit }}
      - -X main.date={{ .Date }}
```
{% endraw %}

### Именование архивов (Intel/Silicon для macOS)

{% raw %}
```yaml
archives:
  - formats: [tar.gz]        # ← НЕ format: (deprecated в v2!)
    format_overrides:
      - goos: windows
        formats: [zip]       # ← НЕ format: zip
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- if and (eq .Os "darwin") (eq .Arch "amd64") }}darwin_amd64_intel
      {{- else if and (eq .Os "darwin") (eq .Arch "arm64") }}darwin_arm64_silicon
      {{- else }}{{ .Os }}_{{ .Arch }}
      {{- end }}
    files:
      - README.md
      - CHANGELOG.md
      - .env.example
      - opencode.jsonc
```
{% endraw %}

Результирующие имена:
```
my-api-mcp_1.0.0_linux_amd64.tar.gz
my-api-mcp_1.0.0_linux_arm64.tar.gz
my-api-mcp_1.0.0_darwin_amd64_intel.tar.gz     ← macOS Intel
my-api-mcp_1.0.0_darwin_arm64_silicon.tar.gz   ← macOS Apple Silicon
my-api-mcp_1.0.0_windows_amd64.zip
checksums.txt
```

---

## Golangci-lint (`.golangci.yml`)

```yaml
linters:
  enable:
    - gofmt      # форматирование с флагом -s (simplify)
    - govet      # статический анализ
    - errcheck   # проверка обработки ошибок
    - staticcheck
    - unused

linters-settings:
  gofmt:
    simplify: true  # эквивалент gofmt -s

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

{: .warning }
> CI использует `gofmt` линтер с флагом `-s` (simplify). **Запускайте `gofmt -s -w .`** локально перед каждым коммитом, не просто `gofmt`. Ручное выравнивание полей структур пробелами вызывает ошибку линтера.

---

## Versioning и релизный процесс

### Semantic Versioning

```
v1.2.3
│ │ └── PATCH: bugfix
│ └──── MINOR: новые tools, обратно совместимо
└────── MAJOR: breaking changes (переименование tools, смена протокола)
```

### Создание релиза

```bash
# 1. Убедиться что CI зелёный
gh run list --limit 5

# 2. Прогнать тесты локально
go test ./... -race

# 3. Обновить CHANGELOG.md
#    [Unreleased] → [v1.1.0] — YYYY-MM-DD

# 4. Закоммитить
git add CHANGELOG.md
git commit -m "chore: release v1.1.0"
git push origin main

# 5. Dry-run горелизера
goreleaser release --snapshot --clean
# Проверить артефакты в dist/

# 6. Создать тег
git tag -a v1.1.0 -m "Release v1.1.0"
git push origin v1.1.0

# 7. Следить за Actions
gh run watch
```

### Makefile targets

```makefile
.PHONY: tag-patch tag-minor tag-major version snapshot

version:
	@git describe --tags --always --dirty 2>/dev/null || echo "no tags"

snapshot:
	goreleaser release --snapshot --clean

tag-patch:
	$(eval CURRENT := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"))
	$(eval PATCH := $(shell echo $(CURRENT) | awk -F. '{print $$1"."$$2"."$$3+1}'))
	@echo "Current: $(CURRENT) → New: $(PATCH)"
	@read -p "Create tag $(PATCH)? [y/N] " yn; \
	  [ "$$yn" = "y" ] && git tag -a $(PATCH) -m "Release $(PATCH)" && git push origin $(PATCH)
```

---

## GitHub Settings

Перед первым релизом настройте репозиторий:

### 1. Workflow permissions

**Settings → Actions → General → Workflow permissions:**
Выбрать **"Read and write permissions"** — иначе goreleaser не сможет создавать Releases.

### 2. Branch protection

**Settings → Branches → Add rule** для `main`:
- ✅ Require status checks: `test`, `lint`
- ✅ Require branches to be up to date
- ✅ Do not allow bypassing

### 3. GitHub Pages (для docs)

**Settings → Pages:**
- Source: **GitHub Actions** (не "Deploy from a branch")

---

## Conventional Commits

Goreleaser генерирует changelog из сообщений коммитов:

| Prefix | Секция в Release Notes |
|--------|----------------------|
| `feat:` | 🚀 New Features |
| `fix:` | 🐛 Bug Fixes |
| `perf:` | ⚡ Performance |
| `refactor:`, `chore:`, `ci:` | 🔧 Improvements |
| `docs:` | 📖 Documentation |
| `test:` | (не попадает в changelog) |

```bash
git commit -m "feat: add myapi_search_users tool"
git commit -m "fix(transport): handle scanner buffer overflow"
git commit -m "feat!: rename all tools to v2 naming"  # ← breaking change → major
```

---

## Что дальше?

- [Решение проблем]({{ site.baseurl }}/troubleshooting) — частые ошибки и их исправление
