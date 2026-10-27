package audit

import (
	"errors"
	"time"

	"go.uber.org/zap"
)

type Event struct {
	Timestamp int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

type Observer interface {
	Update(Event) error
	Close() error
}

type Notifier struct {
	observers []Observer
	logger    *zap.SugaredLogger
}

func NewNotifier(logger *zap.SugaredLogger, observers ...Observer) *Notifier {
	return &Notifier{
		observers: observers,
		logger:    logger,
	}
}

func (n *Notifier) Subscribe(o Observer) {
	n.observers = append(n.observers, o)
}

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
