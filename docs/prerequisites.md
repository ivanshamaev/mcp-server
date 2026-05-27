---
layout: default
title: Предварительные требования
nav_order: 3
permalink: /prerequisites
---

# Предварительные требования
{: .no_toc }

<details open markdown="block">
  <summary>Содержание</summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Go 1.22+

### Проверить версию

```bash
go version
# go version go1.22.4 linux/amd64  ← нужно >= 1.22
```

### Установить без sudo (если Go нет или версия старая)

```bash
cd /tmp
curl -sL https://go.dev/dl/go1.22.4.linux-amd64.tar.gz -o go.tar.gz
mkdir -p ~/.local/go
tar -C ~/.local -xzf go.tar.gz

# Добавить в PATH
echo 'export PATH="$HOME/.local/go/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc

go version  # проверить
```

{: .important }
> В bash-сессиях без `.bashrc` (CI, скрипты) добавляй явно:  
> `export PATH="/home/your-user/.local/go/bin:$PATH"`

### Почему Go 1.22?

- `range` по целым числам: `for i := range 10 { ... }`
- Пакеты `slices`, `maps` из stdlib
- `log/slog` — структурированное логирование

---

## Инструменты разработки

После установки Go установите инструменты разработки:

```bash
# goreleaser — кросс-компиляция и автоматические релизы
go install github.com/goreleaser/goreleaser/v2@latest

# golangci-lint — линтер (включает gofmt -s, govet, errcheck и др.)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# goimports — авто-импорты (опционально, удобен в IDE)
go install golang.org/x/tools/cmd/goimports@latest
```

### Проверить установку

```bash
goreleaser --version    # goreleaser version 2.x.x
golangci-lint --version # golangci-lint has version 1.x.x
```

---

## Git

```bash
git --version  # git version 2.x.x

# Настроить user (если не настроен)
git config --global user.name "Your Name"
git config --global user.email "you@example.com"
```

---

## GitHub CLI (рекомендуется)

```bash
# Установка на Ubuntu/Debian
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
  | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) \
  signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] \
  https://cli.github.com/packages stable main" \
  | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
sudo apt update && sudo apt install gh

# Авторизация
gh auth login
```

---

## API токен

Для вашего API вам понадобится токен доступа. Например, для Yandex Metrika — OAuth токен.

Токен хранится в `.env` файле (не коммитится!):

```bash
# .env
ACCESS_TOKEN=your_token_here
```

```bash
# .env.example (коммитится — шаблон для других разработчиков)
ACCESS_TOKEN=
```

{: .warning }
> `.env` должен быть в `.gitignore`. Никогда не коммитьте реальные токены!

---

## Проверочный чеклист

- [ ] `go version` показывает >= 1.22
- [ ] `goreleaser --version` работает
- [ ] `golangci-lint --version` работает
- [ ] `git --version` работает
- [ ] API токен получен и записан в `.env`

## Что дальше?

- [Структура проекта]({{ site.baseurl }}/project-structure) — как организовать код
