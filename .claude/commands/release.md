# Release — выпустить новую версию

Создай тег и отправь его на GitHub. GitHub Actions автоматически соберёт бинарники и создаст релиз.

## Перед релизом — чеклист

1. Убедись что все тесты проходят:
```bash
export PATH="/home/ivan/.local/go/bin:$PATH"
go test ./... -race -timeout 60s
```

2. Убедись что `main` ветка чистая и актуальная:
```bash
git status           # должно быть "nothing to commit"
git log --oneline -5 # проверь последние коммиты
```

3. Проверь CHANGELOG.md — раздел `[Unreleased]` должен отражать все изменения.

4. Запусти goreleaser dry-run:
```bash
make release-dry-run
```
   Убедись что артефакты собираются корректно в `./dist/`.

## Выбрать тип версии

Посмотри текущую версию: `make version`

Выбери нужный тип по [Semantic Versioning](https://semver.org):

| Команда | Когда использовать | Пример |
|---------|-------------------|--------|
| `make tag-patch` | Bugfix, нет новых features | v1.0.0 → v1.0.1 |
| `make tag-minor` | Новые tools/функции, обратно совместимо | v1.0.1 → v1.1.0 |
| `make tag-major` | Breaking changes (переименование tools, новый протокол) | v1.1.0 → v2.0.0 |

## Обновить CHANGELOG.md перед тегом

Переименуй секцию `[Unreleased]` в новую версию:
```markdown
## [v1.1.0] — 2026-05-27

### Added
- ...

## [Unreleased]
(пусто)

[v1.1.0]: https://github.com/ivanshamaev/mcp-server/compare/v1.0.0...v1.1.0
[Unreleased]: https://github.com/ivanshamaev/mcp-server/compare/v1.1.0...HEAD
```

Закоммить изменения:
```bash
git add CHANGELOG.md
git commit -m "chore: release v1.1.0"
git push origin main
```

## Создать релиз

```bash
make tag-minor    # или tag-patch / tag-major
```

Команда:
1. Покажет текущую и новую версию
2. Запросит подтверждение (y/N)
3. Создаст annotated tag
4. Отправит тег на GitHub
5. GitHub Actions запустит `.github/workflows/release.yml`

## Следить за релизом

```bash
# Посмотреть статус GitHub Actions
gh run list --workflow=release.yml --limit=5

# Следить за логами
gh run watch
```

Или открой: https://github.com/ivanshamaev/mcp-server/actions/workflows/release.yml

## Проверить готовый релиз

```bash
gh release list
gh release view v1.1.0
```

Или открой: https://github.com/ivanshamaev/mcp-server/releases/latest

## Если что-то пошло не так

### Удалить ошибочный тег
```bash
git tag -d v1.1.0               # удалить локально
git push origin :refs/tags/v1.1.0  # удалить на GitHub
# Удалить черновик релиза:
gh release delete v1.1.0 --yes
```

### Пересобрать релиз
После исправления ошибки создай тег заново (инкрементируй patch).

## Артефакты релиза (что получится)

```
yametrika-mcp_v1.1.0_linux_amd64.tar.gz
yametrika-mcp_v1.1.0_linux_arm64.tar.gz
yametrika-mcp_v1.1.0_darwin_amd64.tar.gz
yametrika-mcp_v1.1.0_darwin_arm64.tar.gz
yametrika-mcp_v1.1.0_windows_amd64.zip
checksums.txt                            # SHA256 хэши всех файлов
```

Каждый архив содержит: бинарник, README.md, CHANGELOG.md, .env.example, opencode.jsonc
