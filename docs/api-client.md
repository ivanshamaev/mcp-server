# HTTP клиент

## Шаблон клиента

```go title="internal/myapi/client.go"
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

func WithBaseURL(u string) Option {
    return func(c *Client) { c.baseURL = u }
}

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

!!! tip "Functional options"
    `WithBaseURL` используется в тестах для перенаправления запросов на mock-сервер `httptest.NewServer()`.

---

## Метод get

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

    req.Header.Set("Authorization", "OAuth "+c.token)
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
        // Первые 300 символов тела — часто содержат детали ошибки
        return fmt.Errorf("HTTP %d from %s: %.300s", resp.StatusCode, path, body)
    }

    return json.Unmarshal(body, dst)
}
```

---

## Паттерны аутентификации

=== "OAuth / Yandex"

    ```go
    req.Header.Set("Authorization", "OAuth "+c.token)
    ```

=== "Bearer / JWT"

    ```go
    req.Header.Set("Authorization", "Bearer "+c.token)
    ```

=== "API Key (header)"

    ```go
    req.Header.Set("X-API-Key", c.token)
    // или
    req.Header.Set("Authorization", "ApiKey "+c.token)
    ```

=== "Basic Auth"

    ```go
    import "encoding/base64"
    
    creds := base64.StdEncoding.EncodeToString(
        []byte(c.user + ":" + c.password))
    req.Header.Set("Authorization", "Basic "+creds)
    ```

=== "Query param"

    ```go
    query := url.Values{}
    query.Set("api_key", c.token)
    // передать в get() как query параметр
    ```

---

## API методы

```go title="internal/myapi/users.go"
type User struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
    Email    string `json:"email"`
    Role     string `json:"role"`
    IsActive bool   `json:"is_active"`
}

type usersResponse struct {
    Users []User `json:"users"`
    Total int    `json:"total"`
}

func (c *Client) GetUsers(ctx context.Context) ([]User, error) {
    var resp usersResponse
    if err := c.get(ctx, "/v1/users", nil, &resp); err != nil {
        return nil, fmt.Errorf("GetUsers: %w", err)
    }
    return resp.Users, nil
}

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

## Пагинация

=== "Автосбор всех страниц"

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

=== "Параметры в tool"

    Для больших датасетов — limit/offset как параметры tool:
    
    ```go
    // В buildToolRegistry():
    "limit":  {Type: "string", Description: "Записей на страницу", Default: "100"},
    "offset": {Type: "string", Description: "Смещение", Default: "0"},
    
    // В handler:
    limit  := getStringDefault(args, "limit", "100")
    offset := getStringDefault(args, "offset", "0")
    ```

---

## Rate Limiting

```go
func (c *Client) doWithRetry(req *http.Request) (*http.Response, error) {
    for attempt := 0; attempt < 3; attempt++ {
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

## Правила ошибок

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
