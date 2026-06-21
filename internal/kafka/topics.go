package kafka

// Topic name constants for the JobCrawl event pipeline.
//
// Flow:
//   Crawler → [jobs.raw] → Processor → [jobs.processed] → ES Indexer
//                                                        → Alert Evaluator → [jobs.alerts] → Notifier
const (
	// TopicJobsRaw receives raw crawled job listings from the crawlers.
	// Key: company slug (ensures ordering per company).
	// Value: JSON-encoded CrawlEvent.
	TopicJobsRaw = "jobs.raw"

	// TopicJobsProcessed receives fully processed jobs (deduped, upserted, AI-enriched).
	// Key: job UUID.
	// Value: JSON-encoded ProcessedEvent.
	TopicJobsProcessed = "jobs.processed"

	// TopicJobsAlerts receives alert notifications when new jobs match user rules.
	// Key: user UUID.
	// Value: JSON-encoded AlertEvent.
	TopicJobsAlerts = "jobs.alerts"
)

// Consumer group IDs.
const (
	GroupJobProcessor = "jobcrawl-processor"
	GroupESIndexer    = "jobcrawl-es-indexer"
	GroupAlertEval    = "jobcrawl-alert-evaluator"
	GroupNotifier     = "jobcrawl-notifier"
)
