# Build MCP Server Binary

Собери Go бинарник MCP сервера.

## Шаги

1. Убедись что Go установлен: `go version`
2. Скачай зависимости: `go mod download`
3. Запусти `go vet ./...` для проверки кода
4. Собери бинарник командой:

```bash
CGO_ENABLED=0 go build \
  -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)" \
  -o bin/mcp-server \
  ./cmd/server
```

5. Проверь что бинарник создан: `ls -lh bin/mcp-server`
6. Проверь базовый запуск (должен ждать stdin):
```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' | ./bin/mcp-server
```
   В ответе должен быть JSON с `result.protocolVersion`.

## Возможные проблемы

- **Go не установлен:** `sudo snap install go --classic` или скачай с https://go.dev/dl/
- **Ошибки компиляции:** покажи полный вывод `go build -v ./...`
- **Ошибки зависимостей:** запусти `go mod tidy`

## Результат

Готовый бинарник `./bin/mcp-server`, готовый к использованию в opencode config.
