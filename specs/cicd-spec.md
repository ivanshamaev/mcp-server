# CI/CD Specification

## Обзор

| Триггер | Workflow | Jobs |
|---------|----------|------|
| `push main`, PR в main | `ci.yml` | test (ubuntu+macos) · build · lint · goreleaser-check |
| `push tag v*.*.*` | `release.yml` | release (goreleaser → GitHub Releases) |
| Вручную (GitHub UI) | `release.yml` | release |

Файлы: `.github/workflows/ci.yml`, `.github/workflows/release.yml`

## CI Workflow (`ci.yml`)

### Триггеры

```yaml
on:
  push:
    branches: [main]
    paths-ignore: ["**.md", "docs/**", ".gitignore"]
  pull_request:
    branches: [main]
    paths-ignore: ["**.md", "docs/**", ".gitignore"]
```

Изменения только в `.md` и `.gitignore` не запускают CI — экономия минут раннера.

### Jobs

#### `test` — матрица платформ/версий Go

```
ubuntu-latest + Go 1.22   ← основная конфигурация
ubuntu-latest + Go 1.23   ← проверка совместимости с новой версией
macos-latest  + Go 1.22   ← нативная проверка на macOS (не кросс-компиляция!)
```

> macOS runner важен: кросс-компиляция darwin-бинарника с Ubuntu не гарантирует, что он запустится на реальном Mac (разница в syscall, сигналах, stdio-поведении).

Шаги:
1. `go mod download` + `go mod verify`
2. `go vet ./...`
3. `go test ./... -v -race -timeout 60s`
4. Coverage: `go test ./... -coverprofile=coverage.out`

#### `build` — кросс-компиляция

Все 5 платформ на одном Ubuntu раннере (кросс-компиляция Go работает без дополнительных инструментов):

```
linux/amd64
linux/arm64
darwin/amd64
darwin/arm64
windows/amd64
```

CGO_ENABLED=0 — статическая линковка, нет зависимости от libc целевой системы.

#### `lint` — golangci-lint

```yaml
uses: golangci/golangci-lint-action@v6
with:
  version: v1.59
  args: --timeout=5m
```

Конфиг линтера: `.golangci.yml`. Включены: `gofmt` (с `-s`!), `govet`, `errcheck`, `staticcheck`, `unused` и др.

> **Важно:** `gofmt` линтер использует флаг `-s` (simplify). Ручное выравнивание полей структур пробелами вызывает ошибку линтера. Всегда запускай `gofmt -s -w .` локально перед коммитом.

#### `goreleaser-check` — проверка сборки артефактов

```yaml
uses: goreleaser/goreleaser-action@v6
with:
  args: release --snapshot --clean
```

Собирает все платформы локально на раннере, не публикует. Проверяет что `.goreleaser.yml` валиден и все бинарники компилируются.

> `--snapshot` — сборка без тега и без публикации в GitHub Releases. Использует version_template из `.goreleaser.yml`.

### Node.js версия

```yaml
env:
  FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true
```

`actions/checkout@v4` и `actions/setup-go@v5` внутри используют Node.js 20, который объявлен deprecated. С 2 июня 2026 Node 24 станет default, с 16 сентября 2026 Node 20 будет удалён. Флаг opt-in убирает warning и готовит к принудительному переходу.

## Release Workflow (`release.yml`)

### Триггеры

```yaml
on:
  push:
    tags: ["v*.*.*"]    # автоматический релиз по тегу
  workflow_dispatch:     # ручной запуск через GitHub UI (кнопка "Run workflow")
    inputs:
      tag:
        description: "Tag to release (e.g. v1.0.0)"
        required: true
```

`workflow_dispatch` полезен для:
- Пересборки релиза без удаления и пересоздания тега
- Тестирования процесса релиза

### Permissions

```yaml
permissions:
  contents: write  # создание GitHub Release, загрузка assets
```

`packages: write` — не нужен (не публикуем в GitHub Container Registry).

### Job: release

1. `git checkout` с `fetch-depth: 0` — goreleaser нужен полный history для changelog
2. `go test ./... -race` — тесты перед релизом
3. `goreleaser release --clean` — полная сборка + публикация

## Goreleaser (`.goreleaser.yml`)

### Версия

Используется goreleaser **v2**. Конфиг начинается с `version: 2`.

### Before hooks

```yaml
before:
  hooks:
    - go mod tidy
    - go mod verify
    - go vet ./...
```

Запускаются перед сборкой — дополнительная проверка.

### Builds

```yaml
builds:
  - binary: yametrika-mcp    # имя бинарника в архиве
    main: ./cmd/server
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ignore:
      - goos: windows
        goarch: arm64         # Windows ARM64 не нужен
    ldflags:
      - -s -w
      - -X main.version={{ .Version }}
      - -X main.commit={{ .ShortCommit }}
      - -X main.date={{ .Date }}
```

> Три переменные `version`, `commit`, `date` **должны быть объявлены** в `cmd/server/main.go`:
> ```go
> var (
>     version = "dev"
>     commit  = "none"
>     date    = "unknown"
> )
> ```

### Archives

```yaml
archives:
  - formats: [tar.gz]              # НЕ format: (deprecated в v2)
    format_overrides:
      - goos: windows
        formats: [zip]             # НЕ format: (deprecated в v2)
```

> **Gotcha goreleaser v2:** поля `format` и `format_overrides[].format` переименованы в `formats` (список). Старые имена вызывают ошибку `goreleaser check`.

### Именование архивов

Darwin-архивы имеют суффиксы для ясности:

```yaml
name_template: >-
  {{ .ProjectName }}_{{ .Version }}_
  {{- if and (eq .Os "darwin") (eq .Arch "amd64") }}darwin_amd64_intel
  {{- else if and (eq .Os "darwin") (eq .Arch "arm64") }}darwin_arm64_silicon
  {{- else }}{{ .Os }}_{{ .Arch }}
  {{- end }}
```

Результирующие имена:
```
yametrika-mcp_1.2.0_linux_amd64.tar.gz
yametrika-mcp_1.2.0_linux_arm64.tar.gz
yametrika-mcp_1.2.0_darwin_amd64_intel.tar.gz    ← macOS Intel
yametrika-mcp_1.2.0_darwin_arm64_silicon.tar.gz  ← macOS Apple Silicon
yametrika-mcp_1.2.0_windows_amd64.zip
checksums.txt                                      ← SHA256 всех архивов
```

Содержимое каждого архива: бинарник + `README.md` + `CHANGELOG.md` + `.env.example` + `opencode.jsonc`.

> **Darwin = macOS** — официальное название ядра. `GOOS=darwin` это macOS. `amd64` = Intel, `arm64` = Apple Silicon (M1/M2/M3/M4).

### Changelog (автоматический)

Goreleaser генерирует changelog из коммитов между тегами по Conventional Commits:

| Prefix | Секция в Release Notes |
|--------|----------------------|
| `feat:` | 🚀 New Features |
| `fix:` | 🐛 Bug Fixes |
| `perf:` | ⚡ Performance |
| `refactor:`, `chore:`, `build:`, `ci:` | 🔧 Improvements |
| `docs:` | 📖 Documentation |

Исключаются: Merge commits, `chore(deps):`, коммиты со словом `typo`.

### Snapshot vs Release

| Режим | Команда | Версия | Публикация |
|-------|---------|--------|-----------|
| Snapshot | `goreleaser release --snapshot --clean` | `v1.2.0-snapshot-abc1234` | ❌ только локально в `./dist/` |
| Release | `goreleaser release --clean` | `v1.2.0` (из тега) | ✅ GitHub Releases |

## Versioning

Схема: **Semantic Versioning** `vMAJOR.MINOR.PATCH`

| Тип | Когда | Команда |
|-----|-------|---------|
| `patch` | Bugfix, нет новых features | `make tag-patch` |
| `minor` | Новые tools, обратно совместимо | `make tag-minor` |
| `major` | Breaking changes (переименование tools, смена протокола) | `make tag-major` |

Pre-release суффиксы: `-alpha.1`, `-beta.1`, `-rc.1` → goreleaser автоматически помечает как pre-release.

### Make targets

```bash
make version          # показать текущий тег
make tag-patch        # v1.0.0 → v1.0.1 (интерактивно, с подтверждением)
make tag-minor        # v1.0.1 → v1.1.0
make tag-major        # v1.1.0 → v2.0.0
make snapshot         # локальная сборка всех платформ → ./dist/
make release-dry-run  # goreleaser check + snapshot (без публикации)
```

### Процесс релиза

```bash
# 1. Убедиться что тесты зелёные
go test ./... -race

# 2. Обновить CHANGELOG.md — переименовать [Unreleased] в [v1.1.0]
# 3. Закоммитить
git commit -m "chore: release v1.1.0"
git push origin main

# 4. Проверить dry-run
make release-dry-run

# 5. Создать тег (интерактивно, с подтверждением)
make tag-minor

# 6. Следить за Actions
gh run watch
```

## Известные проблемы и решения

### Gotcha: snapshot job в release workflow

**Проблема:** `snapshot` job с `if: github.event_name == 'pull_request'` в workflow с `on: push: tags: v*` никогда не запускается — trigger и condition несовместимы.

**Решение:** Snapshot job должен быть в `ci.yml` (где есть PR trigger), не в `release.yml`.

### Gotcha: jobs с несовпадающими условиями показываются как пустые

В GitHub Actions UI jobs с несработавшим `if:` отображаются как skipped/empty в Usage tab. Это не ошибка — это ожидаемое поведение. Проверяй что trigger workflow совместим с условиями jobs.

### Gotcha: goreleaser v2 — format → formats

```yaml
# ❌ Старый синтаксис (goreleaser v1, deprecated в v2)
archives:
  - format: tar.gz
    format_overrides:
      - goos: windows
        format: zip

# ✅ Новый синтаксис (goreleaser v2)
archives:
  - formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
```

Проверяй конфиг командой: `goreleaser check`

### Gotcha: .env ключи должны быть UPPER_SNAKE_CASE

```bash
# ❌ Неправильно — godotenv загрузит AccessToken, код читает ACCESS_TOKEN
AccessToken=y0_...

# ✅ Правильно
ACCESS_TOKEN=y0_...
```

`config.go` читает `os.Getenv("ACCESS_TOKEN")`. `godotenv.Load()` загружает ключ как написан. Несовпадение имён = токен не найден = ошибка старта.

### Gotcha: ldflags переменные должны быть объявлены

```go
// ❌ Goreleaser инжектирует -X main.commit=... но переменной нет → молча игнорируется
var version = "dev"

// ✅ Все три переменные объявлены
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```
