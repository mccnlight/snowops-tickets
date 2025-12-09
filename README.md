# Snowops Ticket Service

Сервис тикетов обеспечивает полный цикл управления уборочными заданиями: создание тикета KGU, назначение водителей подрядчиком, фиксация рейсов и нарушений по данным камер, обжалование со стороны водителя. Все данные хранятся в PostgreSQL, авторизация — по JWT из `snowops-auth-service`.

## Возможности

- Жизненный цикл тикета: `PLANNED → IN_PROGRESS → COMPLETED → CLOSED`, отмена (`CANCELLED`) разрешена только до появления фактов (рейсов/фактического старта).
- Гранулярный RBAC:
  - `AKIMAT_ADMIN` — read-only по всем тикетам, рейсам и обжалованиям.
  - `KGU_ZKH_ADMIN` — создание тикетов, отмена, закрытие, удаление, просмотр прогресса.
  - `CONTRACTOR_ADMIN` — управление назначениями, перевод в `IN_PROGRESS`/`COMPLETED`, доступ только к своим тикетам.
  - `DRIVER` — только собственные задания и апелляции.
  - `LANDFILL_ADMIN`, `LANDFILL_USER` — доступ к журналу приёма снега (`/landfill/reception-journal`).
  - `TOO_ADMIN` — доступ запрещён (deprecated, используйте LANDFILL_ADMIN).
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
| `ANPR_SERVICE_URL`     | URL ANPR сервиса для получения событий                             | обязательная (например, `http://anpr-service:8082`)               |
| `ANPR_INTERNAL_TOKEN`  | внутренний токен для запросов к ANPR сервису                       | обязательная                                                      |

## Доменные сущности

- **Ticket** — участок + подрядчик + контракт + плановый период. Никаких нормативов, только фактические данные.
- **TicketAssignment** — связь `ticket ↔ driver ↔ vehicle`, статус отметки водителя (`NOT_STARTED`, `IN_WORK`, `COMPLETED`). Содержит поля `trip_started_at` и `trip_finished_at` для автоматического учета времени рейсов.
- **Trip** — факт рейса от камер (entry/exit LPR и volume события). Статусы: `OK`, `ROUTE_VIOLATION`, `FOREIGN_AREA`, `MISMATCH_PLATE`, `OVER_CAPACITY`, `NO_AREA_WORK`, `NO_ASSIGNMENT`, `SUSPICIOUS_VOLUME`, `OVER_CONTRACT_LIMIT`. Поле `violation_reason` заполняется `snowops-violations-service` для быстрого отображения причины нарушения в карточке тикета. Поля `total_volume_m3` (рассчитанный объем снега) и `auto_created` (флаг автоматического создания) добавлены для автоматического учета рейсов.
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
- `DELETE /kgu/tickets/:id` — удалить тикет (только тикеты, созданные организацией пользователя).
  
  **Поведение:**
  - Удаление тикета каскадно удаляет связанные назначения (`ticket_assignments`) и апелляции (`appeals`)
  - Рейсы (`trips`) остаются, но `ticket_id` становится `NULL` (ON DELETE SET NULL)
  
  **Ответ:** 204 No Content при успехе

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
  - `PUT /driver/assignments/:id/mark-in-work` — установить `IN_WORK` и зафиксировать время начала рейса (`trip_started_at`). Автоматически переведёт тикет в `IN_PROGRESS`, если это первый факт.
  - `PUT /driver/assignments/:id/mark-completed` — установить `COMPLETED` и зафиксировать время окончания рейса (`trip_finished_at`). Автоматически:
    - Рассчитывает объем перевезенного снега на основе событий ANPR за период рейса (суммирует `snow_volume_m3` всех событий въезда)
    - Создает или обновляет запись `Trip` с рассчитанным объемом (`total_volume_m3`) и флагом `auto_created=true`
    - Если расчет объема не удался (ANPR недоступен, события отсутствуют), рейс завершается с объемом 0
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

### LANDFILL (`/landfill`)

- `GET /landfill/reception-journal` — журнал приёма снега (все заезды на полигоны LANDFILL организации).

  **Query параметры:**
  - `polygon_ids` (опционально) — список UUID полигонов через запятую
  - `date_from` (опционально) — начало периода (RFC3339 или YYYY-MM-DD)
  - `date_to` (опционально) — конец периода (RFC3339 или YYYY-MM-DD)
  - `contractor_id` (опционально) — UUID подрядчика для фильтрации
  - `status` (опционально) — статус рейса: `OK`, `ROUTE_VIOLATION`, `FOREIGN_AREA`, и т.д.

  **Доступ:** только `LANDFILL_ADMIN`, `LANDFILL_USER`

  **Ответ:**
  ```json
  {
    "data": {
      "trips": [
        {
          "trip_id": "uuid",
          "entry_at": "2025-01-15T10:30:00Z",
          "exit_at": "2025-01-15T10:45:00Z",
          "polygon_id": "uuid",
          "polygon_name": "Полигон №1",
          "vehicle_plate_number": "KZ 123 ABC",
          "detected_plate_number": "KZ 123 ABC",
          "contractor_id": "uuid",
          "contractor_name": "TOO Snow Demo",
          "detected_volume_entry": 42.5,
          "detected_volume_exit": 2.1,
          "net_volume_m3": 40.4,
          "status": "OK"
        }
      ],
      "total_volume_m3": 1250.8,
      "total_trips": 31
    }
  }
  ```

  **Примечание:** Возвращает только рейсы, где `trip.polygon_id` принадлежит полигонам LANDFILL организации. Для получения списка полигонов используйте `GET /polygons` из `snowops-operations-service` с фильтром по `organization_id`.

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

