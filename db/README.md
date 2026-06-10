# База данных — IfBest Enterprise Видеотека

## Стек

| Компонент | Версия |
|-----------|--------|
| PostgreSQL | 16 (alpine) |
| SQLAlchemy | 2.0+ |
| Alembic | 1.13+ |
| psycopg | 3.1+ |
| Docker Compose | v2 |

## Схема таблицы `videos`

| Столбец | Тип | Ограничения | Описание |
|---------|-----|-------------|----------|
| `id` | `UUID` | PK, DEFAULT gen_random_uuid() | Первичный ключ |
| `title` | `VARCHAR(255)` | NOT NULL | Название видео |
| `file_path` | `VARCHAR(512)` | NOT NULL | Object key в SeaweedFS (напр. `videos/video.mp4`) |
| `views` | `INTEGER` | NOT NULL, DEFAULT 0, CHECK >= 0 | Количество просмотров |
| `created_at` | `TIMESTAMPTZ` | NOT NULL, DEFAULT now() | Дата и время добавления |

## Быстрый старт

### 1. Настройка окружения

Скопируйте `.env.example` в `.env` и заполните переменные:

```bash
cp .env.example .env
# Отредактируйте .env: задайте POSTGRES_USER, POSTGRES_PASSWORD, POSTGRES_DB
```

### 2. Поднять PostgreSQL

```bash
docker compose up -d
# Дождаться статуса healthy:
docker compose ps
```

### 3. Установить зависимости Python

```bash
python -m venv .venv
source .venv/bin/activate      # Linux/macOS
# .venv\Scripts\activate       # Windows
pip install -r requirements.txt
```

### 4. Применить миграции

```bash
alembic upgrade head
```

Для отката:

```bash
alembic downgrade base
```

### 5. Заполнить тестовыми данными

```bash
python -m db.seed
```

После запуска в таблице `videos` появятся 4 записи. Повторный запуск безопасен — дублей не создаёт.

## Структура каталогов

```
VideoLibrary/
├── alembic.ini               # конфигурация Alembic
├── docker-compose.yml        # PostgreSQL 16 сервис
├── requirements.txt          # Python-зависимости
├── .env.example              # шаблон переменных окружения
├── db/
│   ├── __init__.py
│   ├── database.py           # engine и SessionLocal
│   ├── models.py             # ORM-модель Video
│   ├── seed.py               # тестовые данные
│   └── README.md             # этот файл
├── migrations/
│   ├── env.py                # настройка Alembic (читает DATABASE_URL)
│   ├── script.py.mako        # шаблон ревизий
│   └── versions/
│       └── fa11cd8a1aea_create_videos_table.py
└── static/                   # заглушки mp4-файлов (в .gitignore)
```
