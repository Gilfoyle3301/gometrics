package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// URLObserver — приёмник аудита, отправляющий события POST-запросом
// на удалённый URL.
type URLObserver struct {
	url    string
	client *http.Client
}

// NewURLObserver создаёт приёмник для указанного URL с таймаутом запроса
// 10 секунд.
func NewURLObserver(url string) *URLObserver {
	return &URLObserver{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Update отправляет событие на URL в формате JSON; статус ответа 400
// и выше считается ошибкой.
func (o *URLObserver) Update(e Event) error {
	data, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	resp, err := o.client.Post(o.url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("post audit event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("audit endpoint returned %s", resp.Status)
	}
	return nil
}

// Close освобождает простаивающие соединения HTTP-клиента.
func (o *URLObserver) Close() error {
	o.client.CloseIdleConnections()
	return nil
}
