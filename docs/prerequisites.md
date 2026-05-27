# Предварительные требования

## Go 1.22+

=== "Проверить версию"

    ```bash
    go version
    # go version go1.22.4 linux/amd64  ← нужно >= 1.22
    ```

=== "Установить без sudo"

    ```bash
    cd /tmp
    curl -sL https://go.dev/dl/go1.22.4.linux-amd64.tar.gz -o go.tar.gz
    mkdir -p ~/.local/go
    tar -C ~/.local -xzf go.tar.gz

    # Добавить в PATH навсегда
    echo 'export PATH="$HOME/.local/go/bin:$PATH"' >> ~/.bashrc
    source ~/.bashrc

    go version  # проверить
    ```

!!! warning "В bash-сессиях без `.bashrc`"
    В CI, скриптах и некоторых IDE `.bashrc` не загружается. Добавляй явно:
    ```bash
    export PATH="/home/your-user/.local/go/bin:$PATH"
    ```

**Почему Go 1.22?**

- `range` по целым числам: `for i := range 10 { ... }`
- Пакеты `slices`, `maps` из stdlib
- `log/slog` — структурированное логирование (используем для stderr логов)

---

## Инструменты разработки

```bash
# goreleaser — кросс-компиляция и автоматические релизы
go install github.com/goreleaser/goreleaser/v2@latest

# golangci-lint — линтер (включает gofmt -s, govet, errcheck и др.)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# goimports — авто-импорты (опционально, удобен в IDE)
go install golang.org/x/tools/cmd/goimports@latest
```

Проверить:
```bash
goreleaser --version    # goreleaser version 2.x.x
golangci-lint --version # golangci-lint has version 1.x.x
```

---

## Git + GitHub CLI

```bash
# Настроить user (если не настроен)
git config --global user.name "Your Name"
git config --global user.email "you@example.com"

# GitHub CLI (для управления PR и releases)
# Ubuntu/Debian:
sudo apt install gh

# Авторизация
gh auth login
```

---

## API токен

Для вашего API вам понадобится токен доступа. Храните его в `.env`:

```bash title=".env (не коммитится!)"
ACCESS_TOKEN=your_token_here
```

```bash title=".env.example (коммитится — шаблон)"
ACCESS_TOKEN=
# LOG_LEVEL=debug
# LOG_FILE=/tmp/mcp-debug.log
```

!!! danger "Важно"
    `.env` должен быть в `.gitignore`. Никогда не коммитьте реальные токены!

---

## Чеклист

- [x] `go version` показывает >= 1.22
- [x] `goreleaser --version` работает
- [x] `golangci-lint --version` работает
- [x] `git --version` работает
- [x] API токен получен и записан в `.env`
