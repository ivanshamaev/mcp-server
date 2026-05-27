---
layout: default
title: HTTP клиент
nav_order: 7
permalink: /api-client
---

# HTTP клиент
{: .no_toc }

<details open markdown="block">
  <summary>Содержание</summary>
  {: .text-delta }
1. TOC
{:toc}
</details>

---

## Шаблон клиента

Базовый клиент для любого REST API:

```go
// internal/myapi/client.go
package myapi

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "time"
)

const defaultBaseURL = "https://api.example.com"

type Client struct {
    baseURL    string
    token      string
    httpClient *http.Client
}

type Option func(*Client)

// WithBaseURL — переопределить базовый URL (используется в тестах для mock-сервера)
func WithBaseURL(u string) Option {
    return func(c *Client) { c.baseURL = u }
}

// WithTimeout — переопределить таймаут HTTP клиента
func WithTimeout(d time.Duration) Option {
    return func(c *Client) { c.httpClient.Timeout = d }
}

func NewClient(token string, opts ...Option) *Client {
    c := &Client{
        baseURL: defaultBaseURL,
        token:   token,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,  // явный таймаут обязателен
        },
    }
    for _, o := range opts {
        o(c)
    }
    return c
}
```

---

## Метод get

Универсальный GET с JSON декодированием ответа:

```go
func (c *Client) get(ctx context.Context, path string, query url.Values, dst any) error {
    u := c.baseURL + path
    if len(query) > 0 {
        u += "?" + query.Encode()
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
    if err != nil {
        return fmt.Errorf("build request: %w", err)
    }

    // Выберите нужный заголовок для вашего API:
    req.Header.Set("Authorization", "OAuth "+c.token)   // Yandex OAuth
    // req.Header.Set("Authorization", "Bearer "+c.token) // Bearer token
    // req.Header.Set("X-API-Key", c.token)               // API Key
    req.Header.Set("Accept", "application/json")

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("http get %s: %w", path, err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("read body: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        // Включаем первые 300 символов тела — часто там есть детали ошибки
        return fmt.Errorf("HTTP %d from %s: %.300s", resp.StatusCode, path, body)
    }

    return json.Unmarshal(body, dst)
}
```

---

## Паттерны аутентификации

| API тип | Заголовок |
|---------|-----------|
| OAuth (Yandex) | `Authorization: OAuth <token>` |
| Bearer / JWT | `Authorization: Bearer <token>` |
| API Key (header) | `X-API-Key: <key>` |
| API Key (query) | `?api_key=<key>` (добавить в `url.Values`) |
| Basic Auth | `Authorization: Basic base64(user:pass)` |

```go
// Basic Auth
import "encoding/base64"
creds := base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
req.Header.Set("Authorization", "Basic "+creds)

// Query param
query := url.Values{}
query.Set("api_key", c.token)
```

---

## API методы

### GET со структурированным ответом

```go
// internal/myapi/items.go

type Item struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

// Приватный тип для обёртки ответа API
type itemsResponse struct {
    Items []Item `json:"items"`
    Total int    `json:"total"`
}

func (c *Client) GetItems(ctx context.Context) ([]Item, error) {
    var resp itemsResponse
    if err := c.get(ctx, "/v1/items", nil, &resp); err != nil {
        return nil, fmt.Errorf("GetItems: %w", err)
    }
    return resp.Items, nil
}

func (c *Client) GetItem(ctx context.Context, id string) (*Item, error) {
    var resp struct {
        Item Item `json:"item"`
    }
    if err := c.get(ctx, "/v1/items/"+id, nil, &resp); err != nil {
        return nil, fmt.Errorf("GetItem %s: %w", id, err)
    }
    return &resp.Item, nil
}
```

### GET с query параметрами

```go
func (c *Client) GetReport(ctx context.Context, params map[string]string) (*Report, error) {
    query := url.Values{}
    for k, v := range params {
        query.Set(k, v)
    }

    var resp Report
    if err := c.get(ctx, "/v1/report", query, &resp); err != nil {
        return nil, fmt.Errorf("GetReport: %w", err)
    }
    return &resp, nil
}
```

### POST

```go
func (c *Client) post(ctx context.Context, path string, query url.Values, dst any) error {
    u := c.baseURL + path
    if len(query) > 0 {
        u += "?" + query.Encode()
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, nil)
    if err != nil {
        return fmt.Errorf("build request: %w", err)
    }
    req.Header.Set("Authorization", "OAuth "+c.token)

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("http post %s: %w", path, err)
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return fmt.Errorf("read body: %w", err)
    }

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
        return fmt.Errorf("HTTP %d from %s: %.300s", resp.StatusCode, path, body)
    }

    if dst != nil {
        return json.Unmarshal(body, dst)
    }
    return nil
}
```

---

## Пагинация

### Автоматический сбор всех страниц

Для небольших датасетов (< 10 000 записей):

```go
func (c *Client) GetAllItems(ctx context.Context) ([]Item, error) {
    var all []Item
    offset := 0
    const limit = 100

    for {
        query := url.Values{}
        query.Set("limit", fmt.Sprintf("%d", limit))
        query.Set("offset", fmt.Sprintf("%d", offset))

        var page struct {
            Items []Item `json:"items"`
        }
        if err := c.get(ctx, "/v1/items", query, &page); err != nil {
            return nil, fmt.Errorf("GetAllItems page %d: %w", offset/limit, err)
        }

        all = append(all, page.Items...)

        if len(page.Items) < limit {
            break  // последняя страница
        }
        offset += limit
    }

    return all, nil
}
```

### Параметры пагинации в tool

Для больших датасетов — передавать limit/offset как параметры tool:

```go
// Tool объявляет параметры
"limit":  {Type: "string", Description: "Количество записей (1-1000)", Default: "100"},
"offset": {Type: "string", Description: "Смещение для пагинации", Default: "0"},

// Handler передаёт их в API
func (s *Server) toolGetItems(ctx context.Context, args map[string]any) ToolCallResult {
    limit  := getStringDefault(args, "limit", "100")
    offset := getStringDefault(args, "offset", "0")
    // ...
}
```

---

## Rate Limiting

```go
// Retry с экспоненциальной задержкой при HTTP 429
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
    for attempt := 0; attempt < 3; attempt++ {
        // Клонируем тело запроса для retry (если есть)
        resp, err := c.httpClient.Do(req)
        if err != nil {
            return nil, err
        }

        if resp.StatusCode != http.StatusTooManyRequests {
            return resp, nil
        }

        resp.Body.Close()

        delay := time.Duration(1<<attempt) * time.Second  // 1s, 2s, 4s
        select {
        case <-req.Context().Done():
            return nil, req.Context().Err()
        case <-time.After(delay):
        }
    }
    return nil, fmt.Errorf("rate limit exceeded after 3 retries")
}
```

---

## Правила обработки ошибок

```go
// ✅ Всегда оборачивай с контекстом
return nil, fmt.Errorf("GetItems: %w", err)
return nil, fmt.Errorf("GetItem %s: %w", id, err)

// ✅ Sentinel errors для известных кейсов
var ErrNotFound = errors.New("not found")

// ❌ Потеря контекста
return nil, err

// ❌ Запрещено
panic("something went wrong")
```

---

## Что дальше?

- [Добавление инструментов]({{ site.baseurl }}/adding-tools) — как подключить API методы к MCP tools
