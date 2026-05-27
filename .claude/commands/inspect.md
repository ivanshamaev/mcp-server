# MCP Inspector — отладка сервера

Запусти MCP Inspector для интерактивной отладки через веб-интерфейс.

## Шаги

1. Убедись что бинарник собран: `ls -lh bin/mcp-server`, иначе `/build`

2. Загрузи ACCESS_TOKEN:
```bash
export $(grep -v '^#' .env | xargs)
```

3. Запусти MCP Inspector:
```bash
ACCESS_TOKEN="$ACCESS_TOKEN" npx @modelcontextprotocol/inspector ./bin/mcp-server
```

4. Открой в браузере: **http://localhost:5173**

5. В веб-интерфейсе:
   - Вкладка **Tools** → список всех зарегистрированных tools
   - Нажми на tool → форма с параметрами → **Execute**
   - Вкладка **Logs** → все JSON-RPC сообщения в реальном времени

## Если npx недоступен

Установи Node.js: `sudo snap install node --classic` или https://nodejs.org/

Или установи глобально: `npm install -g @modelcontextprotocol/inspector`
Затем: `ACCESS_TOKEN="$ACCESS_TOKEN" mcp-inspector ./bin/mcp-server`

## Альтернатива: ручной режим с логами

```bash
# Запусти сервер с DEBUG логами в файл
LOG_LEVEL=debug LOG_FILE=/tmp/mcp-debug.log ACCESS_TOKEN="$ACCESS_TOKEN" ./bin/mcp-server &
SERVER_PID=$!

# В другом терминале следи за логами
tail -f /tmp/mcp-debug.log

# Отправляй запросы
echo '{"jsonrpc":"2.0","id":1,"method":"initialize",...}' > /proc/$SERVER_PID/fd/0

# Остановка
kill $SERVER_PID
```

## Что проверять

- [ ] Все tools видны в `tools/list`
- [ ] `metrika_get_counters` возвращает список счётчиков
- [ ] `metrika_get_report` с реальным `counter_id` возвращает данные
- [ ] Неверный `counter_id` → `isError: true` с понятным сообщением
- [ ] Неверный метод → JSON-RPC error `-32601`
