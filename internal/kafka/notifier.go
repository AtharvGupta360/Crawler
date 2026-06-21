package kafka

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/AtharvGupta360/JobCrawl/internal/ws"
	kafka "github.com/segmentio/kafka-go"
)

// Notifier consumes jobs.alerts and pushes real-time notifications to
// connected WebSocket clients via the hub.
type Notifier struct {
	reader *kafka.Reader
	hub    *ws.Hub
	logger *slog.Logger

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewNotifier creates and returns a new Notifier.
func NewNotifier(brokers []string, hub *ws.Hub, logger *slog.Logger) *Notifier {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          TopicJobsAlerts,
		GroupID:        GroupNotifier,
		MinBytes:       1e3,
		MaxBytes:       1e6,
		MaxWait:        2 * time.Second,
		CommitInterval: time.Second,
		StartOffset:    kafka.LastOffset,
	})
	return &Notifier{
		reader: reader,
		hub:    hub,
		logger: logger.With("component", "notifier"),
		stopCh: make(chan struct{}),
	}
}

// Start launches the consumer loop in a background goroutine.
func (n *Notifier) Start(ctx context.Context) {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		n.logger.Info("notifier started", "topic", TopicJobsAlerts)
		for {
			select {
			case <-n.stopCh:
				return
			case <-ctx.Done():
				return
			default:
			}

			msg, err := n.reader.ReadMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				n.logger.Error("notifier: read error", "error", err)
				time.Sleep(time.Second)
				continue
			}
			n.handleMessage(msg)
		}
	}()
}

// Stop gracefully shuts down the notifier.
func (n *Notifier) Stop() {
	close(n.stopCh)
	n.wg.Wait()
	n.reader.Close() //nolint:errcheck
	n.logger.Info("notifier stopped")
}

// wsPayload is the JSON shape delivered to the WebSocket client.
type wsPayload struct {
	Type      string    `json:"type"`
	AlertID   string    `json:"alert_id"`
	JobID     string    `json:"job_id"`
	Title     string    `json:"title"`
	Company   string    `json:"company"`
	MatchedAt time.Time `json:"matched_at"`
}

func (n *Notifier) handleMessage(msg kafka.Message) {
	var event AlertEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		n.logger.Error("notifier: unmarshal error", "error", err)
		return
	}

	payload := wsPayload{
		Type:      "job_alert",
		AlertID:   event.AlertID.String(),
		JobID:     event.JobID.String(),
		Title:     event.Title,
		Company:   event.Company,
		MatchedAt: event.MatchedAt,
	}

	n.hub.SendToUser(event.UserID, payload)

	n.logger.Info("notification sent",
		"user_id", event.UserID,
		"job_title", event.Title,
		"company", event.Company,
	)
}
