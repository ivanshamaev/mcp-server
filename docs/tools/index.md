# Инструменты (Tools)

MCP tools — это функции, которые AI-клиент вызывает для получения данных.

<div class="grid cards" markdown>

-   :material-plus-box:{ .lg .middle } **Добавление tool**

    ---

    4 шага: API метод → регистрация → handler → тест.
    С полным примером и паттернами именования.

    [:octicons-arrow-right-24: Пошаговое руководство](../adding-tools.md)

</div>

## Анатомия tool

```python
Tool = {
    "name": "myapi_get_users",        # snake_case, prefix_action_entity
    "description": "...",              # когда и зачем использовать
    "inputSchema": {                   # JSON Schema для параметров
        "type": "object",
        "properties": { ... },
        "required": [...]
    }
}
```

Когда AI-клиент получает список tools, он использует `description` для решения **когда** вызывать каждый инструмент. Хорошее описание = tool будет использоваться правильно.
