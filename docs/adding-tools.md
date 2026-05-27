---
layout: default
title: Добавление инструментов
nav_order: 8
permalink: /adding-tools
---

# Добавление инструментов (tools)
{: .no_toc }

<details open markdown="block">
  <summary>Содержание</summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Обзор процесса

Каждый новый tool — это 4 шага:

```
1. API метод        internal/myapi/items.go       ← GetItems()
2. Регистрация      internal/mcp/server.go         ← buildToolRegistry()
3. Handler          internal/mcp/handlers.go        ← toolGetItems()
4. Тест             internal/myapi/items_test.go    ← TestGetItems
```

Разберём на конкретном примере: добавляем `myapi_get_users`.

---

## Шаг 1: API метод

Создайте файл `internal/myapi/users.go`:

```go
package myapi

import (
    "context"
    "fmt"
)

// User описывает пользователя.
// Поля называйте по json-тегам из документации API.
type User struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    Email    string `json:"email"`
    Role     string `json:"role"`
    IsActive bool   `json:"is_active"`
}

// usersResponse — обёртка ответа API.
// Делаем приватной — снаружи пакета не нужна.
type usersResponse struct {
    Users []User `json:"users"`
    Total int    `json:"total"`
}

// GetUsers возвращает всех пользователей.
func (c *Client) GetUsers(ctx context.Context) ([]User, error) {
    var resp usersResponse
    if err := c.get(ctx, "/v1/users", nil, &resp); err != nil {
        return nil, fmt.Errorf("GetUsers: %w", err)
    }
    return resp.Users, nil
}

// GetUser возвращает пользователя по ID.
func (c *Client) GetUser(ctx context.Context, id string) (*User, error) {
    var resp struct {
        User User `json:"user"`
    }
    if err := c.get(ctx, "/v1/users/"+id, nil, &resp); err != nil {
        return nil, fmt.Errorf("GetUser %s: %w", id, err)
    }
    return &resp.User, nil
}
```

---

## Шаг 2: Регистрация в tool registry

В файле `internal/mcp/server.go` добавьте в `buildToolRegistry()`:

```go
func (s *Server) buildToolRegistry() []Tool {
    return []Tool{
        // ... существующие tools ...

        {
            Name: "myapi_get_users",
            Description: "Получить список всех пользователей системы. " +
                "Используй для поиска пользователей, просмотра ролей и статусов.",
            InputSchema: InputSchema{
                Type:       "object",
                Properties: map[string]Property{},  // нет обязательных параметров
            },
        },

        {
            Name: "myapi_get_user",
            Description: "Получить детальную информацию о конкретном пользователе по его ID. " +
                "Используй когда знаешь ID пользователя из myapi_get_users.",
            InputSchema: InputSchema{
                Type: "object",
                Properties: map[string]Property{
                    "user_id": {
                        Type:        "string",
                        Description: "ID пользователя (числовой, напр. \"42\")",
                    },
                },
                Required: []string{"user_id"},
            },
        },
    }
}
```

---

## Шаг 3: Реализация handler

В файле `internal/mcp/handlers.go`:

### Добавить case в `executeTool`

```go
func (s *Server) executeTool(ctx context.Context, name string, args map[string]any) ToolCallResult {
    switch name {
    // ... существующие cases ...
    case "myapi_get_users":
        return s.toolGetUsers(ctx)
    case "myapi_get_user":
        return s.toolGetUser(ctx, args)
    default:
        return errorContent(fmt.Sprintf("unknown tool: %s", name))
    }
}
```

### Реализовать методы

```go
func (s *Server) toolGetUsers(ctx context.Context) ToolCallResult {
    users, err := s.myapi.GetUsers(ctx)
    if err != nil {
        return errorContent("Ошибка получения пользователей: " + err.Error())
    }
    result, _ := jsonText(users)
    return result
}

func (s *Server) toolGetUser(ctx context.Context, args map[string]any) ToolCallResult {
    id := getString(args, "user_id")
    if id == "" {
        return errorContent("параметр user_id обязателен")
    }

    user, err := s.myapi.GetUser(ctx, id)
    if err != nil {
        return errorContent(fmt.Sprintf("Ошибка получения пользователя %s: %s", id, err))
    }

    result, _ := jsonText(user)
    return result
}
```

### Вспомогательные функции

```go
// getString извлекает строковый параметр из args (или "" если отсутствует)
func getString(args map[string]any, key string) string {
    v, _ := args[key].(string)
    return v
}

// getStringDefault возвращает параметр или значение по умолчанию
func getStringDefault(args map[string]any, key, def string) string {
    if v, ok := args[key].(string); ok && v != "" {
        return v
    }
    return def
}

// jsonText сериализует любое значение в ToolCallResult с отступами
func jsonText(v any) (ToolCallResult, error) {
    b, err := json.MarshalIndent(v, "", "  ")
    if err != nil {
        return errorContent("failed to serialize response: " + err.Error()), err
    }
    return textContent(string(b)), nil
}
```

---

## Шаг 4: Тест

```go
// internal/myapi/users_test.go
package myapi

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestGetUsers(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Проверяем авторизацию
        if r.Header.Get("Authorization") != "OAuth test-token" {
            t.Errorf("unexpected auth header: %q", r.Header.Get("Authorization"))
        }
        // Проверяем путь
        if r.URL.Path != "/v1/users" {
            t.Errorf("unexpected path: %q", r.URL.Path)
        }

        json.NewEncoder(w).Encode(usersResponse{
            Users: []User{
                {ID: 1, Name: "Alice", Email: "alice@example.com", Role: "admin"},
                {ID: 2, Name: "Bob", Email: "bob@example.com", Role: "user"},
            },
        })
    }))
    defer ts.Close()

    client := NewClient("test-token", WithBaseURL(ts.URL))
    users, err := client.GetUsers(context.Background())
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(users) != 2 {
        t.Errorf("want 2 users, got %d", len(users))
    }
    if users[0].Name != "Alice" {
        t.Errorf("want Alice, got %q", users[0].Name)
    }
}

func TestGetUsers_APIError(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusUnauthorized)
        w.Write([]byte(`{"error": "invalid token"}`))
    }))
    defer ts.Close()

    client := NewClient("bad-token", WithBaseURL(ts.URL))
    _, err := client.GetUsers(context.Background())
    if err == nil {
        t.Fatal("expected error, got nil")
    }
    t.Logf("error (expected): %v", err)  // убедиться что сообщение читаемое
}
```

---

## Правила хорошего описания tool

AI-клиент использует `Description` для решения **когда** вызывать tool. Плохое описание = tool не будет использоваться:

```go
// ❌ Бесполезное описание
Description: "Get users"

// ✅ Хорошее описание
Description: "Получить список всех пользователей. " +
    "Используй для поиска пользователей, просмотра их ролей, статусов активности. " +
    "Возвращает id, name, email, role, is_active для каждого пользователя."
```

Чеклист для описания:
- [ ] Что делает tool (одно предложение)
- [ ] Когда его использовать
- [ ] Какие данные возвращает
- [ ] Связь с другими tools ("используй id из myapi_get_users")

### Описание параметров

```go
// ❌ Плохо
"date1": {Type: "string", Description: "Start date"},

// ✅ Хорошо
"date1": {
    Type:        "string",
    Description: "Начало периода в формате YYYY-MM-DD или относительная дата: today, yesterday, 7daysAgo, 30daysAgo",
    Default:     "7daysAgo",
},
```

### Enum для фиксированного набора значений

```go
"status": {
    Type:        "string",
    Description: "Фильтр по статусу пользователя",
    Enum:        []string{"active", "inactive", "blocked"},
},
```

---

## Именование tools

Формат: `{api_prefix}_{action}_{entity}` или `{api_prefix}_{action}_{entity}s`

```
myapi_get_users         ← получить список
myapi_get_user          ← получить один объект (без 's')
myapi_create_user       ← создать
myapi_update_user       ← обновить
myapi_delete_user       ← удалить
myapi_search_users      ← поиск
myapi_get_user_orders   ← вложенный ресурс
```

Правила:
- Всегда `snake_case`
- Один чёткий глагол: get, create, update, delete, list, search
- Не сокращай (`get_u` вместо `get_users` — плохо)

---

## Цепочка обработки ошибок

```
HTTP 404 от API
    ↓
client.get() возвращает:
    fmt.Errorf("HTTP 404 from /v1/users/999: {\"error\":\"not found\"}")
    ↓
GetUser() оборачивает:
    fmt.Errorf("GetUser 999: %w", err)
    ↓
toolGetUser() перехватывает:
    errorContent("Ошибка получения пользователя 999: GetUser 999: HTTP 404 ...")
    ↓
handleToolsCall() оборачивает в okResponse (НЕ errorResponse!):
    {"result": {"content": [{"type":"text","text":"Ошибка..."}], "isError": true}}
```

---

## Что дальше?

- [Тестирование]({{ site.baseurl }}/testing) — unit тесты и E2E тест протокола
