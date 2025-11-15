# Snowops Ticket Service

Сервис тикетов обеспечивает полный цикл управления уборочными заданиями: создание тикета KGU, назначение водителей подрядчиком, фиксация рейсов и нарушений по данным камер, обжалование со стороны водителя. Все данные хранятся в PostgreSQL, авторизация — по JWT из `snowops-auth-service`.

## Возможности

- Жизненный цикл тикета: `PLANNED → IN_PROGRESS → COMPLETED → CLOSED`, отмена (`CANCELLED`) разрешена только до появления фактов (рейсов/фактического старта).
- Гранулярный RBAC:
  - `AKIMAT_ADMIN` — read-only по всем тикетам, рейсам и обжалованиям.
  - `KGU_ZKH_ADMIN` — создание тикетов, отмена, закрытие, просмотр прогресса.
  - `CONTRACTOR_ADMIN` — управление назначениями, перевод в `IN_PROGRESS`/`COMPLETED`, доступ только к своим тикетам.
  - `DRIVER` — только собственные задания и апелляции.
  - `TOO_ADMIN` — доступ запрещён.
- Автоматическое обновление статусов по фактам: первый рейс или отметка водителя переводит тикет в `IN_PROGRESS`, закрытие всех рейсов + отметки водителей переводят в `COMPLETED`.
- Trip ingestion:
  - Привязка рейса к тикету по `ticket_assignment` (driver/vehicle). Если сопоставить нельзя, рейс сохраняется со статусом `NO_ASSIGNMENT`.
- Контроль нарушений (`ROUTE_VIOLATION`, `FOREIGN_AREA`, `MISMATCH_PLATE`, `OVER_CAPACITY`, `NO_AREA_WORK`, `NO_ASSIGNMENT`, `SUSPICIOUS_VOLUME`, `OVER_CONTRACT_LIMIT`) и отображение бейджа `has_violations` + `violation_reason`.
- Апелляции водителей по рейсам: подача, просмотр, комментарии, обновление статусов KGU/Акиматом.

## Требования

- Go 1.23+
- PostgreSQL 15+

## Запуск локально

```bash
# поднять Postgres
cd deploy
docker compose up -d

# запустить сервис
cd ..
APP_ENV=development \
DB_DSN="postgres://postgres:postgres@localhost:5435/tickets_db?sslmode=disable" \
JWT_ACCESS_SECRET="secret-key" \
HTTP_PORT=8080 \
go run ./cmd/ticket-service
```

## Переменные окружения

| Переменная             | Описание                                                            | Значение по умолчанию                                             |
|------------------------|---------------------------------------------------------------------|--------------------------------------------------------------------|
| `APP_ENV`              | окружение (`development`, `production`)                             | `development`                                                     |
| `HTTP_HOST` / `HTTP_PORT` | адрес HTTP-сервера                                              | `0.0.0.0` / `8080`                                                |
| `DB_DSN`               | строка подключения к PostgreSQL                                     | обязательная                                                      |
| `DB_MAX_OPEN_CONNS`    | максимум одновременных соединений                                   | `25`                                                              |
| `DB_MAX_IDLE_CONNS`    | максимум соединений в пуле                                          | `10`                                                              |
| `DB_CONN_MAX_LIFETIME` | TTL соединения                                                     | `1h`                                                              |
| `JWT_ACCESS_SECRET`    | секрет для проверки JWT                                            | обязательная                                                      |

## Доменные сущности

- **Ticket** — участок + подрядчик + контракт + плановый период. Никаких нормативов, только фактические данные.
- **TicketAssignment** — связь `ticket ↔ driver ↔ vehicle`, статус отметки водителя (`NOT_STARTED`, `IN_WORK`, `COMPLETED`).
- **Trip** — факт рейса от камер (entry/exit LPR и volume события). Статусы: `OK`, `ROUTE_VIOLATION`, `FOREIGN_AREA`, `MISMATCH_PLATE`, `OVER_CAPACITY`, `NO_AREA_WORK`, `NO_ASSIGNMENT`, `SUSPICIOUS_VOLUME`, `OVER_CONTRACT_LIMIT`. Поле `violation_reason` заполняется `snowops-violations-service` для быстрого отображения причины нарушения в карточке тикета.
- **Appeal** — апелляция водителя по рейсу (`SUBMITTED → UNDER_REVIEW → NEED_INFO → APPROVED/REJECTED → CLOSED`).

## API

Все маршруты (кроме `/healthz`) требуют `Authorization: Bearer <jwt>`. Ответы оборачиваются в `{"data": ...}`.

### Health

- `GET /healthz` — проверка работоспособности.

### Акимат (`/akimat`)

- `GET /akimat/tickets` — список всех тикетов с фильтрами `status`, `contractor_id`, `cleaning_area_id`, `contract_id`, `planned_start_from/to`, `planned_end_from/to`, `fact_start_from/to`, `fact_end_from/to`.
- `GET /akimat/tickets/:id` — карточка тикета с метриками, назначениями, рейсами и обжалованиями (read-only).

### KGU (`/kgu`)

- `GET /kgu/tickets` — тикеты, созданные организацией KGU.
- `POST /kgu/tickets` — создать тикет (все поля обязательны).
  ```json
  {
    "cleaning_area_id": "uuid",
    "contractor_id": "uuid",
    "contract_id": "uuid",
    "planned_start_at": "2025-01-01T08:00:00Z",
    "planned_end_at": "2025-01-03T20:00:00Z",
    "description": "ночная уборка"
  }
  ```
- `GET /kgu/tickets/:id` — карточка тикета.
- `PUT /kgu/tickets/:id/cancel` — отменить тикет (доступно только если нет рейсов и `fact_start_at = null`).
- `PUT /kgu/tickets/:id/close` — перевести `COMPLETED → CLOSED` после проверки.

### Подрядчик (`/contractor`)

- `GET /contractor/tickets` — тикеты, где `ticket.contractor_id == org_id`.
- `GET /contractor/tickets/:id` — детали тикета.
- `PUT /contractor/tickets/:id/complete` — перевести в `COMPLETED`, если:
  - все рейсы имеют exit события и пустой кузов на выезде;
  - все активные назначения отмечены `COMPLETED`.
- Управление назначениями:
  - `POST /contractor/tickets/:id/assignments`
    ```json
    { "driver_id": "uuid", "vehicle_id": "uuid" }
    ```
  - `GET /contractor/tickets/:id/assignments`
  - `DELETE /contractor/assignments/:id`
  > Создавать/удалять назначения можно только в статусах `PLANNED` и `IN_PROGRESS`.

### Водитель (`/driver`)

- `GET /driver/tickets` — тикеты, где у водителя есть активное назначение.
- `GET /driver/tickets/:id` — карточка тикета, фильтрована по рейсам/назначениям конкретного водителя.
- Обновление статуса назначения:
  - `PUT /driver/assignments/:id/mark-in-work` — установить `IN_WORK` (автоматически переведёт тикет в `IN_PROGRESS`, если это первый факт).
  - `PUT /driver/assignments/:id/mark-completed` — установить `COMPLETED`.
- Апелляции:
  - `POST /driver/appeals`
    ```json
    {
      "trip_id": "uuid",
      "appeal_reason_type": "ERROR_CAMERA",
      "comment": "номер распознан неверно"
    }
    ```
  - `GET /driver/appeals?ticket_id=` — список собственных апелляций (опционально фильтр по тикету).
  - `GET /driver/appeals/:id`
  - `POST /driver/appeals/:id/comments` — комментарий к апелляции.
  - `GET /driver/appeals/:id/comments`

### Общие форматы

- **TicketDetails (`GET /tickets/:id`)**
  ```json
  {
    "data": {
      "ticket": { "...": "..." },
      "metrics": {
        "total_trips": 5,
        "total_volume_m3": 210.4,
        "has_violations": true
      },
      "assignments": [ ... ],
      "trips": [ ... ],
      "appeals": [ ... ]
    }
  }
  ```
- Каждый объект в `trips` содержит `violation_reason`, если сервис нарушений зафиксировал и пояснил проблему.
- **Ошибки**
  ```json
  { "error": "описание" }
  ```
  - 400 — некорректный ввод (`ErrInvalidInput`).
  - 401 — нет/неверный токен.
  - 403 — недостаточно прав (`ErrPermissionDenied`).
  - 404 — ресурс не найден (`ErrNotFound`).
  - 409 — конфликт статуса/доступа (`ErrConflict`).

## Интеграция с другими сервисами

- `contract_id` в тикетах обязателен; внешний `snowops-contract-service` читает `tickets` и `trips` напрямую через БД и/или REST.
- `cleaning_area_id` и `contractor_id` должны совпадать с записями `snowops-operations-service` и `snowops-roles`.
- Trip ingestion вызывает `TicketService.OnTripCreated` и обновляет usage; сторонние сервисы (LPR/volume) должны дергать внутренний `TripService` (gRPC/крон) или напрямую писать в БД через сервис.

## Тестирование

```bash
go test ./...
```

