// Package audit реализует аудит запросов по паттерну «Наблюдатель»:
// после успешного сохранения метрик событие аудита рассылается
// во все подключённые приёмники (файл, удалённый URL).
package audit

import (
	"errors"
	"time"

	"go.uber.org/zap"
)

// Event — событие аудита: unix-время, имена полученных метрик
// и IP-адрес входящего запроса.
type Event struct {
	Timestamp int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// Observer — приёмник событий аудита (файл, удалённый URL и т.п.).
type Observer interface {
	Update(Event) error
	Close() error
}

// Notifier — субъект паттерна «Наблюдатель»: формирует события аудита
// и рассылает их всем подписанным приёмникам.
type Notifier struct {
	observers []Observer
	logger    *zap.SugaredLogger
}

// NewNotifier создаёт нотификатор с исходным набором приёмников.
// logger может быть nil.
func NewNotifier(logger *zap.SugaredLogger, observers ...Observer) *Notifier {
	return &Notifier{
		observers: observers,
		logger:    logger,
	}
}

// Subscribe добавляет приёмник событий аудита.
func (n *Notifier) Subscribe(o Observer) {
	n.observers = append(n.observers, o)
}

// Notify формирует событие из имён метрик и IP-адреса клиента и отправляет
// его во все приёмники. Вызов на nil-нотификаторе безопасен (аудит отключён);
// ошибки приёмников логируются и не влияют на обработку запроса.
func (n *Notifier) Notify(metricNames []string, ipAddress string) {
	if n == nil {
		return
	}

	event := Event{
		Timestamp: time.Now().Unix(),
		Metrics:   metricNames,
		IPAddress: ipAddress,
	}

	for _, o := range n.observers {
		if err := o.Update(event); err != nil && n.logger != nil {
			n.logger.Errorw("audit observer failed", "error", err)
		}
	}
}

// Close закрывает все приёмники и возвращает объединённую ошибку.
// Вызов на nil-нотификаторе безопасен.
func (n *Notifier) Close() error {
	if n == nil {
		return nil
	}

	var errs []error
	for _, o := range n.observers {
		if err := o.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
