package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type URLObserver struct {
	url    string
	client *http.Client
}

func NewURLObserver(url string) *URLObserver {
	return &URLObserver{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

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

func (o *URLObserver) Close() error {
	o.client.CloseIdleConnections()
	return nil
}
