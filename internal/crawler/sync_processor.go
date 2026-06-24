package crawler

import (
	"context"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
)

// SyncProcessor handles enrichment, alert evaluation, and ES indexing
// for a single job synchronously. Used when Kafka is not configured.
type SyncProcessor interface {
	ProcessSync(ctx context.Context, job *models.Job, isNew bool) error
}
