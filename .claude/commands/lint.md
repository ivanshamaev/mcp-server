# Lint — статический анализ Go кода

Запусти полный набор линтеров.

## Шаги

1. Базовая проверка (встроена в Go):
```bash
go vet ./...
```

2. Форматирование:
```bash
# Проверить (без изменений)
gofmt -l .

# Применить
gofmt -w .
```

3. Если установлен golangci-lint:
```bash
golangci-lint run ./...
```

4. Если golangci-lint не установлен, установи:
```bash
# Linux/Mac
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.59.1

# Или через Go
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

5. Проверь импорты:
```bash
# Установи goimports если нет
go install golang.org/x/tools/cmd/goimports@latest

# Применить
goimports -w .
```

## Автофикс

```bash
# Всё сразу
gofmt -w . && goimports -w . && go vet ./...
```

## Конфиг линтера

Файл `.golangci.yml` в корне проекта управляет правилами.
Добавь новые линтеры по мере необходимости, не убирай существующие.

## Критические ошибки (блокируют коммит)

- `go vet` ошибки
- `errcheck` — необработанные ошибки
- `unused` — неиспользуемый код
- Любой вывод в `os.Stdout` кроме JSON-RPC (нарушает MCP протокол!)
