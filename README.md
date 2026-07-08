# Tarantool Chat RAG

RAG-ассистент по истории Telegram-чата Tarantool (~200 000 сообщений).
Пользователь задаёт вопрос → сервис находит релевантные сообщения → LLM формирует ответ с ссылками на `msg_id`.

## Что это

**RAG** (Retrieval-Augmented Generation) — сначала поиск по чату, потом генерация ответа на основе найденного контекста.

```
Вопрос → BM25 + оконный поиск → Reranker → LLM → ответ + message_ids
```

## Требования

- Go 1.23+
- Файлы данных: `data/tarantool.json`, `data/dataset.json`
- API-ключ для внешних моделей

## Быстрый старт

```bash
# 1. Конфиг
cp .env.example .env
# Укажите API_KEY в .env

# 2. Построить индекс (~15–20 сек)
go run ./cmd/build-index

# 3. Проверить API (опционально)
go run ./cmd/probe-apis

# 4. Запустить сервер
go run ./cmd/server
```

Откройте в браузере: **http://localhost:8080/**

## Конфигурация (.env)

```env
API_KEY=your-token-here

LLM_API_BASE=https://cute.hacode.ru/vllm/llm/v1
LLM_MODEL=lovedheart/Qwen3.5-9B-FP8

EMBEDDING_API_BASE=https://cute.hacode.ru/vllm/embedding/v1
EMBEDDING_MODEL=Qwen/Qwen3-Embedding-0.6B

RERANKER_API_BASE=https://cute.hacode.ru/vllm/reranker/v1
RERANKER_MODEL=nvidia/llama-nemotron-rerank-1b-v2

SUMMARIZER_API_BASE=https://cute.hacode.ru/vllm/summarizer/v1
SUMMARIZER_MODEL=cute-team/teams-summarizator-granite-2B

TARANTOOL_JSON=data/tarantool.json
DATASET_JSON=data/dataset.json
INDEX_DIR=data/index

TOP_K=30
RETRIEVE_CANDIDATES=100
CONTEXT_MESSAGES=20
SUMMARIZE_MIN_CHARS=12000
WINDOW_SIZE=40
WINDOW_STEP=5
NEIGHBOR_RADIUS=25
HTTP_ADDR=:8080
```

## API

| Метод | Endpoint    | Описание                           |
| ---------- | ----------- | ------------------------------------------ |
| GET        | `/`       | Веб-UI для демо                  |
| GET        | `/health` | Статус сервиса                |
| POST       | `/search` | Только поиск сообщений |
| POST       | `/ask`    | Поиск + ответ LLM                |

### Примеры

```bash
curl http://localhost:8080/health

curl -X POST http://localhost:8080/search \
  -H "Content-Type: application/json" \
  -d '{"query": "oracle odbc tarantool"}'

curl -X POST http://localhost:8080/ask \
  -H "Content-Type: application/json" \
  -d '{"question": "как организовать кластер Tarantool 3?"}'
```

## Оценка качества

```bash
go run ./cmd/evaluate
```

Прогоняет 100 вопросов из `data/dataset.json`, результат в `data/eval_results.json`.

| Метрика | Описание                                                                     |
| -------------- | ------------------------------------------------------------------------------------ |
| `hit@30`     | Доля вопросов, где найдено ≥1 нужное сообщение |
| `recall@30`  | Доля всех эталонных`message_id`, попавших в top-30       |
| `mrr`        | Позиция первого релевантного сообщения            |

## Структура проекта

```
cmd/
  server/        HTTP API + веб-UI
  build-index/   Построение BM25-индекса
  evaluate/      Оценка на dataset.json
  probe-apis/    Проверка внешних API

internal/
  config/        Настройки из .env
  loader/        Парсинг tarantool.json, синонимы запросов
  index/         BM25, оконный поиск, RRF
  rag/           Pipeline: Retrieve + Ask
  vllm/          HTTP-клиенты (LLM, reranker, summarizer, embedding)
  api/           HTTP handlers
  eval/          Метрики recall / hit / MRR

data/
  tarantool.json     Экспорт Telegram-чата
  dataset.json       100 тестовых вопросов
  index/             Сохранённый индекс
  eval_results.json  Результаты evaluation
```

## Pipeline (кратко)

1. **ExpandQuery** — синонимы (`oracle` → `оракл`, `odbc`, `jdbc`)
2. **BM25** — поиск по сообщениям и по окнам тредов
3. **RRF** — объединение результатов
4. **Reranker** — переранжирование top-60
5. **Neighbors** — подтягивание ±25 соседних msg_id
6. **Summarizer** — сжатие длинного контекста
7. **LLM** — финальный ответ

## Документация

Подробные PDF-гайды: `docs/Project-Guide-Full.pdf`, `docs/RAG-Theory-Guide.pdf`

Пересоздать PDF:

```bash
python scripts/generate_project_guide_pdf.py
```

## Внешние API

| Сервис | Назначение                                                                    |
| ------------ | --------------------------------------------------------------------------------------- |
| LLM          | Генерация ответа                                                         |
| Reranker     | Переранжирование кандидатов                                   |
| Summarizer   | Сжатие контекста                                                         |
| Embedding    | Векторизация (клиент готов, в pipeline не подключён) |

Формат: OpenAI-compatible (`/chat/completions`, `/rerank`, `/embeddings`).
Авторизация: `Authorization: Bearer <API_KEY>`.
