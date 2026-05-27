# Test MCP Protocol

Прогони полный тест MCP протокола: initialize → tools/list → tools/call.

## Шаги

1. Убедись что бинарник собран (`./bin/mcp-server` существует), иначе запусти `/build`
2. Загрузи переменные из `.env`:
```bash
export $(grep -v '^#' .env | xargs)
```

3. Запусти последовательность MCP сообщений:

```bash
{
  printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0.0"}}}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","method":"notifications/initialized"}\n'
  sleep 0.1
  printf '{"jsonrpc":"2.0","id":2,"method":"tools/list"}\n'
  sleep 0.2
  printf '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"metrika_get_counters","arguments":{}}}\n'
  sleep 2
} | ACCESS_TOKEN="$ACCESS_TOKEN" ./bin/mcp-server
```

4. Проверь ответы:
   - Ответ на id:1 содержит `result.protocolVersion` == `"2024-11-05"`
   - Ответ на id:2 содержит `result.tools` — массив tool объектов
   - Ответ на id:3 содержит `result.content[0].text` с данными счётчиков

5. Запусти unit тесты:
```bash
go test ./... -v -timeout 30s
```

6. Запусти тесты с покрытием:
```bash
go test ./... -coverprofile=coverage.out && go tool cover -html=coverage.out -o coverage.html
```

## Проверка конкретного tool

Замени в шаге 3 последний запрос на нужный tool. Например, для отчёта:
```json
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"metrika_get_report","arguments":{"counter_id":"<ID>","metrics":"ym:s:visits","date1":"7daysAgo","date2":"today"}}}
```

## Ожидаемый результат

Каждый JSON-RPC запрос с `id` получает ответ с тем же `id`.
Notifications (без `id`) — игнорируются сервером, ответа нет.
Ошибки API Metrika → `result.isError: true` (не JSON-RPC error!).
