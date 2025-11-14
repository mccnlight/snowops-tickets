# Инструкция по проверке работы ticket-service

## Предварительные требования

1. Установленный Go (версия 1.23 или выше)
2. PostgreSQL 15 (можно использовать Docker Compose)
3. Insomnia или другой HTTP-клиент для тестирования API
4. JWT токен для аутентификации (получить из auth-service)

## Шаг 1: Подготовка окружения

### 1.1. Запуск базы данных

#### Вариант 1: Использование Docker Compose (рекомендуется)

**Важно:** Убедитесь, что Docker Desktop установлен и запущен на вашем компьютере.

Используйте Docker Compose для запуска PostgreSQL:

```bash
docker-compose up -d
```

**Если вы получили ошибку:**
```
unable to get image 'postgres:15': error during connect: in the default daemon configuration on Windows, the docker client must be run with elevated privileges to connect
```

**Решения:**
1. Убедитесь, что Docker Desktop установлен и запущен
2. Проверьте статус Docker: откройте Docker Desktop и убедитесь, что он работает
3. Перезапустите Docker Desktop
4. Запустите PowerShell от имени администратора и попробуйте снова

Проверьте, что база данных запущена:

```bash
docker-compose ps
```

#### Вариант 2: Установка PostgreSQL напрямую (без Docker)

Если Docker недоступен, установите PostgreSQL напрямую:

1. **Скачайте PostgreSQL 15:**
   - Перейдите на https://www.postgresql.org/download/windows/
   - Скачайте установщик и установите PostgreSQL

2. **Создайте базу данных:**
   ```sql
   -- Подключитесь к PostgreSQL через psql или pgAdmin
   CREATE DATABASE snowops_tickets;
   ```

3. **Настройте подключение:**
   - Убедитесь, что PostgreSQL слушает на порту 5432
   - Проверьте, что пользователь `postgres` существует и имеет пароль `postgres` (или обновите `DB_DSN` в `app.env`)

4. **Обновите `app.env`:**
   ```env
   DB_DSN=postgres://postgres:postgres@localhost:5432/snowops_tickets?sslmode=disable
   ```

### 1.2. Настройка переменных окружения

Убедитесь, что файл `app.env` содержит правильные настройки:

```env
APP_ENV=development
HTTP_HOST=0.0.0.0
HTTP_PORT=8080
DB_DSN=postgres://postgres:postgres@localhost:5432/snowops_tickets?sslmode=disable
JWT_ACCESS_SECRET=dev-secret-change-me-in-production
```

## Шаг 2: Проверка миграций

### 2.1. Запуск сервиса

Запустите сервис для автоматического выполнения миграций:

```bash
go run ./cmd/ticket-service
```

Сервис автоматически выполнит все миграции при старте. Проверьте логи - не должно быть ошибок миграций.

### 2.2. Проверка миграций в базе данных

Подключитесь к базе данных и проверьте созданные таблицы:

```bash
psql -h localhost -U postgres -d snowops_tickets
```

Выполните следующие SQL-запросы для проверки:

```sql
-- Проверка расширений
SELECT * FROM pg_extension WHERE extname IN ('uuid-ossp', 'pgcrypto', 'postgis');

-- Проверка типов ENUM
SELECT typname FROM pg_type WHERE typname IN ('ticket_status', 'trip_status', 'appeal_status', 'violation_type');

-- Проверка таблиц
SELECT table_name 
FROM information_schema.tables 
WHERE table_schema = 'public' 
ORDER BY table_name;

-- Должны быть следующие таблицы:
-- - tickets
-- - ticket_assignments
-- - trips
-- - lpr_events
-- - volume_events
-- - appeals
-- - appeal_comments

-- Проверка индексов
SELECT indexname, tablename 
FROM pg_indexes 
WHERE schemaname = 'public' 
ORDER BY tablename, indexname;

-- Проверка триггеров
SELECT trigger_name, event_object_table 
FROM information_schema.triggers 
WHERE trigger_schema = 'public';
```

### 2.3. Проверка структуры таблицы tickets

```sql
-- Проверка структуры таблицы tickets
\d tickets

-- Проверка ограничений
SELECT 
    conname AS constraint_name,
    contype AS constraint_type,
    pg_get_constraintdef(oid) AS definition
FROM pg_constraint
WHERE conrelid = 'tickets'::regclass;
```

## Шаг 3: Проверка health endpoint

### 3.1. Проверка через curl

```bash
curl http://localhost:8080/healthz
```

Ожидаемый ответ:

```json
{
  "status": "ok"
}
```

### 3.2. Проверка через Insomnia

1. Создайте новый запрос GET
2. URL: `http://localhost:8080/healthz`
3. Отправьте запрос
4. Ожидаемый статус: `200 OK`
5. Ожидаемый ответ: `{"status": "ok"}`

## Шаг 4: Получение JWT токена

Для тестирования защищенных endpoints вам понадобится JWT токен. Получите его из auth-service:

```bash
# Пример запроса к auth-service (замените на реальный endpoint)
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "password"
  }'
```

Сохраните полученный `access_token` для использования в следующих запросах.

## Шаг 5: Тестирование API endpoints через Insomnia

### 5.1. Настройка переменных в Insomnia

Создайте переменные окружения в Insomnia:

- `base_url`: `http://localhost:8080`
- `token`: `<ваш_jwt_токен>`

### 5.2. Создание тикета (POST /akimat/tickets)

**Запрос:**
- Метод: `POST`
- URL: `{{base_url}}/akimat/tickets`
- Headers:
  - `Authorization`: `Bearer {{token}}`
  - `Content-Type`: `application/json`
- Body (JSON):
```json
{
  "cleaning_area_id": "550e8400-e29b-41d4-a716-446655440000",
  "contractor_id": "550e8400-e29b-41d4-a716-446655440001",
  "planned_start_at": "2024-01-15T08:00:00Z",
  "planned_end_at": "2024-01-15T18:00:00Z",
  "description": "Очистка территории от снега"
}
```

**Ожидаемый ответ:**
- Статус: `201 Created`
- Body:
```json
{
  "data": {
    "id": "550e8400-e29b-41d4-a716-446655440002",
    "cleaning_area_id": "550e8400-e29b-41d4-a716-446655440000",
    "contractor_id": "550e8400-e29b-41d4-a716-446655440001",
    "created_by_org_id": "...",
    "status": "PLANNED",
    "planned_start_at": "2024-01-15T08:00:00Z",
    "planned_end_at": "2024-01-15T18:00:00Z",
    "description": "Очистка территории от снега",
    "created_at": "...",
    "updated_at": "..."
  }
}
```

**Сохраните `id` созданного тикета для следующих запросов.**

### 5.3. Получение списка тикетов (GET /akimat/tickets)

**Запрос:**
- Метод: `GET`
- URL: `{{base_url}}/akimat/tickets`
- Headers:
  - `Authorization`: `Bearer {{token}}`

**С фильтрами:**
- URL: `{{base_url}}/akimat/tickets?status=PLANNED&contractor_id=550e8400-e29b-41d4-a716-446655440001`

**Ожидаемый ответ:**
- Статус: `200 OK`
- Body:
```json
{
  "data": [
    {
      "id": "...",
      "cleaning_area_id": "...",
      "contractor_id": "...",
      "status": "PLANNED",
      ...
    }
  ]
}
```

### 5.4. Получение тикета по ID (GET /akimat/tickets/:id)

**Запрос:**
- Метод: `GET`
- URL: `{{base_url}}/akimat/tickets/{{ticket_id}}`
- Headers:
  - `Authorization`: `Bearer {{token}}`

**Ожидаемый ответ:**
- Статус: `200 OK`
- Body:
```json
{
  "data": {
    "id": "...",
    "cleaning_area_id": "...",
    "contractor_id": "...",
    "status": "PLANNED",
    ...
  }
}
```

### 5.5. Тестирование для разных ролей

#### 5.5.1. KGU (КГУ)

**Создание тикета:**
- Метод: `POST`
- URL: `{{base_url}}/kgu/tickets`
- Headers и Body аналогичны запросу для акимата

**Получение списка:**
- Метод: `GET`
- URL: `{{base_url}}/kgu/tickets`

#### 5.5.2. Contractor (Подрядчик)

**Получение списка (только чтение):**
- Метод: `GET`
- URL: `{{base_url}}/contractor/tickets`

**Попытка создания (должна вернуть 403):**
- Метод: `POST`
- URL: `{{base_url}}/contractor/tickets`
- Ожидаемый статус: `403 Forbidden`

#### 5.5.3. Driver (Водитель)

**Получение списка:**
- Метод: `GET`
- URL: `{{base_url}}/driver/tickets`

### 5.6. Тестирование ошибок

#### 5.6.1. Запрос без токена

**Запрос:**
- Метод: `GET`
- URL: `{{base_url}}/akimat/tickets`
- Без заголовка Authorization

**Ожидаемый ответ:**
- Статус: `401 Unauthorized`
- Body:
```json
{
  "error": "authorization header missing"
}
```

#### 5.6.2. Запрос с невалидным токеном

**Запрос:**
- Метод: `GET`
- URL: `{{base_url}}/akimat/tickets`
- Headers:
  - `Authorization`: `Bearer invalid-token`

**Ожидаемый ответ:**
- Статус: `401 Unauthorized`
- Body:
```json
{
  "error": "invalid token"
}
```

#### 5.6.3. Запрос несуществующего тикета

**Запрос:**
- Метод: `GET`
- URL: `{{base_url}}/akimat/tickets/00000000-0000-0000-0000-000000000000`
- Headers:
  - `Authorization`: `Bearer {{token}}`

**Ожидаемый ответ:**
- Статус: `404 Not Found` или `500 Internal Server Error` (в зависимости от реализации)

#### 5.6.4. Создание тикета с невалидными данными

**Запрос:**
- Метод: `POST`
- URL: `{{base_url}}/akimat/tickets`
- Headers:
  - `Authorization`: `Bearer {{token}}`
  - `Content-Type`: `application/json`
- Body (JSON):
```json
{
  "cleaning_area_id": "invalid-uuid",
  "contractor_id": "550e8400-e29b-41d4-a716-446655440001"
}
```

**Ожидаемый ответ:**
- Статус: `400 Bad Request`
- Body:
```json
{
  "error": "invalid input"
}
```

## Шаг 6: Проверка логов

Проверьте логи сервиса на наличие ошибок:

```bash
# Логи должны показывать:
# - Успешное подключение к БД
# - Успешное выполнение миграций
# - Запуск сервера на порту 8080
# - Обработанные HTTP запросы
```

## Шаг 7: Проверка базы данных после операций

После создания тикета проверьте его в базе данных:

```sql
-- Просмотр созданных тикетов
SELECT 
    id,
    cleaning_area_id,
    contractor_id,
    status,
    planned_start_at,
    planned_end_at,
    created_at
FROM tickets
ORDER BY created_at DESC
LIMIT 10;
```

## Чек-лист проверки

- [ ] База данных запущена и доступна
- [ ] Миграции выполнены успешно (все таблицы созданы)
- [ ] Health endpoint возвращает `200 OK`
- [ ] JWT токен получен и валиден
- [ ] Создание тикета работает (POST /akimat/tickets)
- [ ] Получение списка тикетов работает (GET /akimat/tickets)
- [ ] Получение тикета по ID работает (GET /akimat/tickets/:id)
- [ ] Фильтрация по статусу работает
- [ ] Фильтрация по contractor_id работает
- [ ] Проверка прав доступа работает (403 для contractor при создании)
- [ ] Обработка ошибок работает (401, 400, 404)
- [ ] Логи не содержат ошибок
- [ ] Данные корректно сохраняются в БД

## Дополнительные тесты

### Тест производительности

Для проверки производительности можно использовать Apache Bench или wrk:

```bash
# Установка wrk (если не установлен)
# Windows: choco install wrk
# Linux: sudo apt-get install wrk

# Тест health endpoint
wrk -t4 -c100 -d30s http://localhost:8080/healthz

# Тест защищенного endpoint (с токеном)
wrk -t4 -c100 -d30s -H "Authorization: Bearer YOUR_TOKEN" http://localhost:8080/akimat/tickets
```

## Устранение проблем

### Проблема: Docker не запускается

**Ошибка:**
```
unable to get image 'postgres:15': error during connect: in the default daemon configuration on Windows, the docker client must be run with elevated privileges to connect
```

**Решения:**
1. **Установите Docker Desktop:**
   - Скачайте с https://www.docker.com/products/docker-desktop/
   - Установите и перезагрузите компьютер
   - Запустите Docker Desktop и дождитесь полной загрузки

2. **Проверьте, что Docker работает:**
   ```powershell
   docker ps
   ```
   Если команда работает, Docker запущен корректно

3. **Запустите PowerShell от имени администратора:**
   - Правой кнопкой на PowerShell → "Запуск от имени администратора"
   - Попробуйте `docker-compose up -d` снова

4. **Альтернатива:** Используйте установку PostgreSQL напрямую (см. раздел 1.1, Вариант 2)

### Проблема: Миграции не выполняются

**Решение:**
1. Проверьте подключение к БД в `app.env`
2. Убедитесь, что PostgreSQL запущен
3. Проверьте логи сервиса на наличие ошибок
4. Попробуйте выполнить миграции вручную через psql
5. Убедитесь, что база данных `snowops_tickets` существует

### Проблема: 401 Unauthorized

**Решение:**
1. Проверьте, что токен валиден и не истек
2. Убедитесь, что заголовок Authorization имеет формат: `Bearer <token>`
3. Проверьте, что `JWT_ACCESS_SECRET` совпадает с секретом в auth-service

### Проблема: 500 Internal Server Error

**Решение:**
1. Проверьте логи сервиса
2. Убедитесь, что база данных доступна
3. Проверьте корректность данных в запросе
4. Убедитесь, что все зависимости установлены (`go mod download`)

### Проблема: Тикеты не создаются

**Решение:**
1. Проверьте права доступа пользователя (должен быть AKIMAT_ADMIN или KGU_ADMIN)
2. Убедитесь, что UUID в запросе валидны
3. Проверьте формат дат (должен быть RFC3339)
4. Проверьте логи на наличие ошибок валидации

