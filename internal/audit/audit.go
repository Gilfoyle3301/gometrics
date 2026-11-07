// Package audit реализует аудит запросов сервера метрик: события
// обрабатываются по паттерну «наблюдатель» и доставляются в подключённые
// приёмники (файл, URL).
package audit

import (
	"errors"
	"sync"
	"time"

	"go.uber.org/zap"
)

// eventQueueSize — ёмкость очереди событий аудита: Notify кладёт событие
// в буфер, поэтому при переполнении событие отбрасывается, а не блокирует
// запросную горутину.
const eventQueueSize = 1024

// Event — событие аудита: время запроса в Unix-секундах, имена обновлённых
// метрик и IP-адрес клиента.
type Event struct {
	Timestamp int64    `json:"ts"`
	Metrics   []string `json:"metrics"`
	IPAddress string   `json:"ip_address"`
}

// Observer — приёмник событий аудита. Update обрабатывает одно событие,
// Close освобождает ресурсы приёмника.
type Observer interface {
	Update(Event) error
	Close() error
}

// Notifier рассылает события аудита подключённым наблюдателям из отдельной
// горутины, поэтому Notify не блокирует обработчик запроса. Состав
// наблюдателей фиксируется при создании и после него не меняется.
// Вызовы Notify и Close на nil-Notifier безопасны: так аудит отключается.
type Notifier struct {
	observers []Observer
	logger    *zap.SugaredLogger
	events    chan Event
	quit      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewNotifier создаёт нотификатор с указанными наблюдателями и запускает
// горутину рассылки событий. logger может быть nil — ошибки наблюдателей
// в этом случае не логируются.
func NewNotifier(logger *zap.SugaredLogger, observers ...Observer) *Notifier {
	n := &Notifier{
		observers: observers,
		logger:    logger,
		events:    make(chan Event, eventQueueSize),
		quit:      make(chan struct{}),
	}

	n.wg.Add(1)
	go n.run()

	return n
}

func (n *Notifier) run() {
	defer n.wg.Done()

	for {
		select {
		case event := <-n.events:
			n.dispatch(event)
		case <-n.quit:
			for {
				select {
				case event := <-n.events:
					n.dispatch(event)
				default:
					return
				}
			}
		}
	}
}

func (n *Notifier) dispatch(event Event) {
	for _, o := range n.observers {
		if err := o.Update(event); err != nil && n.logger != nil {
			n.logger.Errorw("audit observer failed", "error", err)
		}
	}
}

// Notify ставит в очередь событие аудита об обновлении метрик metricNames
// клиентом ipAddress. При переполненной очереди событие отбрасывается
// с предупреждением в лог.
func (n *Notifier) Notify(metricNames []string, ipAddress string) {
	if n == nil {
		return
	}

	event := Event{
		Timestamp: time.Now().Unix(),
		Metrics:   metricNames,
		IPAddress: ipAddress,
	}

	select {
	case n.events <- event:
	default:
		if n.logger != nil {
			n.logger.Warnw("audit event queue full, event dropped",
				"metrics", event.Metrics, "ip_address", event.IPAddress)
		}
	}
}

// Close останавливает горутину рассылки, дожидается обработки уже
// поставленных в очередь событий и закрывает всех наблюдателей, объединяя
// ошибки. Повторный вызов возвращает nil.
func (n *Notifier) Close() error {
	if n == nil {
		return nil
	}

	var err error
	n.closeOnce.Do(func() {
		close(n.quit)
		n.wg.Wait()

		var errs []error
		for _, o := range n.observers {
			if cerr := o.Close(); cerr != nil {
				errs = append(errs, cerr)
			}
		}
		err = errors.Join(errs...)
	})
	return err
}
