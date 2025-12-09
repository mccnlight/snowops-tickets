package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"ticket-service/internal/config"
	"ticket-service/internal/utils"
)

type ANPREvent struct {
	ID              string    `json:"id"`
	NormalizedPlate string    `json:"normalized_plate"`
	EventTime       time.Time `json:"event_time"`
	Direction       *string   `json:"direction,omitempty"`
	SnowVolumeM3    *float64  `json:"snow_volume_m3,omitempty"`
	CameraID        string    `json:"camera_id"`
	PolygonID       *string   `json:"polygon_id,omitempty"`
}

type ANPREventsResponse struct {
	Data []ANPREvent `json:"data"`
}

type ANPRClient struct {
	baseURL       string
	internalToken string
	httpClient    *http.Client
}

func NewANPRClient(cfg *config.Config) *ANPRClient {
	return &ANPRClient{
		baseURL:       cfg.ExternalServices.ANPRServiceURL,
		internalToken: cfg.ExternalServices.ANPRInternalToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetEventsByPlateAndTime получает события ANPR за указанный период
func (c *ANPRClient) GetEventsByPlateAndTime(ctx context.Context, plate string, startTime, endTime time.Time, direction *string) ([]ANPREvent, error) {
	// Проверяем, что baseURL настроен
	if c.baseURL == "" {
		return nil, fmt.Errorf("ANPR service URL is not configured")
	}

	// Нормализуем номер
	normalizedPlate := utils.NormalizePlate(plate)
	if normalizedPlate == "" {
		return nil, fmt.Errorf("invalid plate number")
	}

	// Формируем URL
	u, err := url.Parse(c.baseURL + "/internal/anpr/events")
	if err != nil {
		return nil, fmt.Errorf("invalid ANPR service URL: %w", err)
	}

	// Добавляем query параметры
	q := u.Query()
	q.Set("plate", normalizedPlate)
	q.Set("start_time", startTime.Format(time.RFC3339))
	q.Set("end_time", endTime.Format(time.RFC3339))
	if direction != nil && *direction != "" {
		q.Set("direction", *direction)
	}
	u.RawQuery = q.Encode()

	// Создаем запрос
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Добавляем internal token
	if c.internalToken != "" {
		req.Header.Set("X-Internal-Token", c.internalToken)
	}

	// Выполняем запрос с retry при сетевых ошибках
	var resp *http.Response
	var lastErr error
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		resp, lastErr = c.httpClient.Do(req)
		if lastErr == nil {
			break
		}
		// Если это последняя попытка, возвращаем ошибку
		if attempt == maxRetries-1 {
			return nil, fmt.Errorf("failed to execute request after %d attempts: %w", maxRetries, lastErr)
		}
		// Ждем перед повторной попыткой (exponential backoff)
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
		// Пересоздаем запрос для повторной попытки
		req, _ = http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if c.internalToken != "" {
			req.Header.Set("X-Internal-Token", c.internalToken)
		}
	}
	if resp == nil {
		return nil, fmt.Errorf("failed to execute request: %w", lastErr)
	}
	defer resp.Body.Close()

	// Читаем ответ
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Проверяем статус код
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ANPR service returned status %d: %s", resp.StatusCode, string(body))
	}

	// Парсим ответ
	var response ANPREventsResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil
}
