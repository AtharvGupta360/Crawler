package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AtharvGupta360/JobCrawl/internal/models"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
)

const jobIndexName = "jobs"

// ElasticStore wraps the Elasticsearch client for job search and indexing.
type ElasticStore struct {
	client *elasticsearch.Client
	logger *slog.Logger
}

// NewElasticStore creates a new Elasticsearch store and verifies the connection.
func NewElasticStore(elasticURL string, logger *slog.Logger) (*ElasticStore, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{elasticURL},
	}

	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating ES client: %w", err)
	}

	// Verify connection
	res, err := client.Info()
	if err != nil {
		return nil, fmt.Errorf("connecting to Elasticsearch: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("Elasticsearch info returned error: %s", res.String())
	}

	logger.Info("Elasticsearch connected", "url", elasticURL)

	return &ElasticStore{client: client, logger: logger}, nil
}

// EnsureJobIndex creates the jobs index with optimized mappings if it doesn't exist.
func (es *ElasticStore) EnsureJobIndex(ctx context.Context) error {
	// Check if index exists
	res, err := es.client.Indices.Exists([]string{jobIndexName})
	if err != nil {
		return fmt.Errorf("checking index existence: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		es.logger.Info("Elasticsearch index already exists", "index", jobIndexName)
		return nil
	}

	// Create index with mappings
	mapping := map[string]any{
		"settings": map[string]any{
			"number_of_shards":   1,
			"number_of_replicas": 0,
			"analysis": map[string]any{
				"analyzer": map[string]any{
					"job_analyzer": map[string]any{
						"type":      "custom",
						"tokenizer": "standard",
						"filter":    []string{"lowercase", "stop", "snowball"},
					},
				},
			},
		},
		"mappings": map[string]any{
			"properties": map[string]any{
				"title": map[string]any{
					"type":     "text",
					"analyzer": "job_analyzer",
					"fields": map[string]any{
						"keyword": map[string]any{"type": "keyword"},
					},
				},
				"normalized_title": map[string]any{
					"type":     "text",
					"analyzer": "job_analyzer",
				},
				"description_clean": map[string]any{
					"type":     "text",
					"analyzer": "job_analyzer",
				},
				"location": map[string]any{
					"type":     "text",
					"analyzer": "standard",
					"fields": map[string]any{
						"keyword": map[string]any{"type": "keyword"},
					},
				},
				"location_type":  map[string]any{"type": "keyword"},
				"employment_type": map[string]any{"type": "keyword"},
				"seniority_level": map[string]any{"type": "keyword"},
				"department": map[string]any{
					"type":     "text",
					"fields": map[string]any{
						"keyword": map[string]any{"type": "keyword"},
					},
				},
				"company_name": map[string]any{
					"type":     "text",
					"fields": map[string]any{
						"keyword": map[string]any{"type": "keyword"},
					},
				},
				"company_slug":    map[string]any{"type": "keyword"},
				"company_id":      map[string]any{"type": "keyword"},
				"skills":          map[string]any{"type": "keyword"},
				"salary_min":      map[string]any{"type": "integer"},
				"salary_max":      map[string]any{"type": "integer"},
				"salary_currency": map[string]any{"type": "keyword"},
				"apply_url":       map[string]any{"type": "keyword", "index": false},
				"source_url":      map[string]any{"type": "keyword", "index": false},
				"ai_summary": map[string]any{
					"type":     "text",
					"analyzer": "job_analyzer",
				},
				"first_seen_at": map[string]any{"type": "date"},
				"last_seen_at":  map[string]any{"type": "date"},
				"is_active":     map[string]any{"type": "boolean"},
			},
		},
	}

	body, err := json.Marshal(mapping)
	if err != nil {
		return fmt.Errorf("marshaling index mapping: %w", err)
	}

	res, err = es.client.Indices.Create(
		jobIndexName,
		es.client.Indices.Create.WithBody(bytes.NewReader(body)),
		es.client.Indices.Create.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("creating index: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("index creation failed: %s", res.String())
	}

	es.logger.Info("Elasticsearch index created", "index", jobIndexName)
	return nil
}

// JobDocument is the Elasticsearch document structure for a job.
type JobDocument struct {
	Title            string   `json:"title"`
	NormalizedTitle  string   `json:"normalized_title,omitempty"`
	DescriptionClean string  `json:"description_clean,omitempty"`
	Location         string   `json:"location,omitempty"`
	LocationType     string   `json:"location_type,omitempty"`
	EmploymentType   string   `json:"employment_type,omitempty"`
	SeniorityLevel   string   `json:"seniority_level,omitempty"`
	Department       string   `json:"department,omitempty"`
	CompanyName      string   `json:"company_name,omitempty"`
	CompanySlug      string   `json:"company_slug,omitempty"`
	CompanyID        string   `json:"company_id"`
	Skills           []string `json:"skills,omitempty"`
	SalaryMin        *int     `json:"salary_min,omitempty"`
	SalaryMax        *int     `json:"salary_max,omitempty"`
	SalaryCurrency   string   `json:"salary_currency,omitempty"`
	ApplyURL         string   `json:"apply_url"`
	SourceURL        string   `json:"source_url"`
	AISummary        string   `json:"ai_summary,omitempty"`
	FirstSeenAt      string   `json:"first_seen_at"`
	LastSeenAt       string   `json:"last_seen_at"`
	IsActive         bool     `json:"is_active"`
}

// JobToDocument converts a Job model (with Company populated) to an ES document.
func JobToDocument(j *models.Job) JobDocument {
	doc := JobDocument{
		Title:            j.Title,
		NormalizedTitle:  j.NormalizedTitle,
		DescriptionClean: j.DescriptionClean,
		Location:         j.Location,
		LocationType:     j.LocationType,
		EmploymentType:   j.EmploymentType,
		SeniorityLevel:   j.SeniorityLevel,
		Department:       j.Department,
		CompanyID:        j.CompanyID.String(),
		SalaryMin:        j.SalaryMin,
		SalaryMax:        j.SalaryMax,
		SalaryCurrency:   j.SalaryCurrency,
		ApplyURL:         j.ApplyURL,
		SourceURL:        j.SourceURL,
		AISummary:        j.AISummary,
		FirstSeenAt:      j.FirstSeenAt.Format("2006-01-02T15:04:05Z"),
		LastSeenAt:       j.LastSeenAt.Format("2006-01-02T15:04:05Z"),
		IsActive:         j.IsActive,
	}

	if j.Company != nil {
		doc.CompanyName = j.Company.Name
		doc.CompanySlug = j.Company.Slug
	}

	// Flatten skills into a string array for keyword faceting
	for _, s := range j.SkillsRequired {
		doc.Skills = append(doc.Skills, s.Name)
	}
	for _, s := range j.SkillsPreferred {
		doc.Skills = append(doc.Skills, s.Name)
	}

	return doc
}

// IndexJob indexes or updates a single job document in Elasticsearch.
func (es *ElasticStore) IndexJob(ctx context.Context, job *models.Job) error {
	doc := JobToDocument(job)

	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshaling job document: %w", err)
	}

	res, err := es.client.Index(
		jobIndexName,
		bytes.NewReader(body),
		es.client.Index.WithDocumentID(job.ID.String()),
		es.client.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("indexing job %s: %w", job.ID, err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("index response error for job %s: %s", job.ID, res.String())
	}

	return nil
}

// BulkIndexJobs indexes multiple jobs in a single bulk request.
func (es *ElasticStore) BulkIndexJobs(ctx context.Context, jobs []*models.Job) error {
	if len(jobs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, job := range jobs {
		// Action line
		meta := map[string]any{
			"index": map[string]any{
				"_index": jobIndexName,
				"_id":    job.ID.String(),
			},
		}
		metaLine, _ := json.Marshal(meta)
		buf.Write(metaLine)
		buf.WriteByte('\n')

		// Document line
		doc := JobToDocument(job)
		docLine, _ := json.Marshal(doc)
		buf.Write(docLine)
		buf.WriteByte('\n')
	}

	res, err := es.client.Bulk(
		bytes.NewReader(buf.Bytes()),
		es.client.Bulk.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("bulk indexing: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("bulk index response error: %s", res.String())
	}

	es.logger.Debug("bulk indexed jobs", "count", len(jobs))
	return nil
}

// DeleteJob removes a job document from the index.
func (es *ElasticStore) DeleteJob(ctx context.Context, jobID uuid.UUID) error {
	res, err := es.client.Delete(
		jobIndexName,
		jobID.String(),
		es.client.Delete.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("deleting job %s: %w", jobID, err)
	}
	defer res.Body.Close()

	// 404 is acceptable (already deleted)
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("delete response error for job %s: %s", jobID, res.String())
	}

	return nil
}

// SearchQuery represents a search request with optional filters.
type SearchQuery struct {
	Query          string `json:"q"`
	SeniorityLevel string `json:"seniority,omitempty"`
	LocationType   string `json:"location_type,omitempty"`
	Company        string `json:"company,omitempty"`
	Department     string `json:"department,omitempty"`
	Limit          int    `json:"limit"`
	Offset         int    `json:"offset"`
}

// SearchResult represents the search response.
type SearchResult struct {
	Jobs       []SearchHit        `json:"jobs"`
	Total      int64              `json:"total"`
	Facets     map[string][]Facet `json:"facets,omitempty"`
}

// SearchHit represents a single search result with highlights.
type SearchHit struct {
	ID         string          `json:"id"`
	Score      float64         `json:"score"`
	Job        JobDocument     `json:"job"`
	Highlights map[string][]string `json:"highlights,omitempty"`
}

// Facet represents a term aggregation bucket.
type Facet struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// SearchJobs performs a full-text + filtered search against the jobs index.
func (es *ElasticStore) SearchJobs(ctx context.Context, sq SearchQuery) (*SearchResult, error) {
	// Build the query
	must := []map[string]any{}
	filter := []map[string]any{
		{"term": map[string]any{"is_active": true}},
	}

	// Full-text query
	if sq.Query != "" {
		must = append(must, map[string]any{
			"multi_match": map[string]any{
				"query":  sq.Query,
				"fields": []string{"title^3", "description_clean", "company_name^2", "department", "ai_summary", "location"},
				"type":   "best_fields",
				"fuzziness": "AUTO",
			},
		})
	}

	// Filters
	if sq.SeniorityLevel != "" {
		filter = append(filter, map[string]any{
			"term": map[string]any{"seniority_level": sq.SeniorityLevel},
		})
	}
	if sq.LocationType != "" {
		filter = append(filter, map[string]any{
			"term": map[string]any{"location_type": sq.LocationType},
		})
	}
	if sq.Company != "" {
		filter = append(filter, map[string]any{
			"term": map[string]any{"company_slug": sq.Company},
		})
	}
	if sq.Department != "" {
		filter = append(filter, map[string]any{
			"match": map[string]any{"department": sq.Department},
		})
	}

	// If no must clauses, use match_all
	if len(must) == 0 {
		must = append(must, map[string]any{"match_all": map[string]any{}})
	}

	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must":   must,
				"filter": filter,
			},
		},
		"highlight": map[string]any{
			"fields": map[string]any{
				"title":            map[string]any{},
				"description_clean": map[string]any{"fragment_size": 150, "number_of_fragments": 2},
				"company_name":     map[string]any{},
			},
			"pre_tags":  []string{"<mark>"},
			"post_tags": []string{"</mark>"},
		},
		"aggs": map[string]any{
			"seniority": map[string]any{
				"terms": map[string]any{"field": "seniority_level", "size": 10},
			},
			"location_type": map[string]any{
				"terms": map[string]any{"field": "location_type", "size": 10},
			},
			"companies": map[string]any{
				"terms": map[string]any{"field": "company_name.keyword", "size": 20},
			},
		},
		"from": sq.Offset,
		"size": sq.Limit,
		"sort": []map[string]any{
			{"_score": map[string]any{"order": "desc"}},
			{"first_seen_at": map[string]any{"order": "desc"}},
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshaling search query: %w", err)
	}

	res, err := es.client.Search(
		es.client.Search.WithContext(ctx),
		es.client.Search.WithIndex(jobIndexName),
		es.client.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("executing search: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.String())
	}

	// Parse response
	var esResp esSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}

	result := &SearchResult{
		Total:  esResp.Hits.Total.Value,
		Facets: make(map[string][]Facet),
	}

	// Map hits
	for _, hit := range esResp.Hits.Hits {
		var doc JobDocument
		if err := json.Unmarshal(hit.Source, &doc); err != nil {
			continue
		}

		searchHit := SearchHit{
			ID:         hit.ID,
			Score:      hit.Score,
			Job:        doc,
			Highlights: hit.Highlight,
		}
		result.Jobs = append(result.Jobs, searchHit)
	}

	// Map aggregations
	for aggName, agg := range esResp.Aggregations {
		var buckets []Facet
		for _, bucket := range agg.Buckets {
			key, ok := bucket.Key.(string)
			if !ok {
				continue
			}
			if strings.TrimSpace(key) == "" {
				continue
			}
			buckets = append(buckets, Facet{
				Key:   key,
				Count: bucket.DocCount,
			})
		}
		result.Facets[aggName] = buckets
	}

	return result, nil
}

// HealthCheck verifies the Elasticsearch connection.
func (es *ElasticStore) HealthCheck(ctx context.Context) error {
	res, err := es.client.Cluster.Health(
		es.client.Cluster.Health.WithContext(ctx),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("ES cluster health: %s", res.String())
	}
	return nil
}

// Close is a no-op for the ES client (no persistent connections to close).
func (es *ElasticStore) Close() {
	es.logger.Info("Elasticsearch store closed")
}

// ─────────────────────────────────────────────
// Internal ES response types
// ─────────────────────────────────────────────

type esSearchResponse struct {
	Hits         esHits                     `json:"hits"`
	Aggregations map[string]esAggregation   `json:"aggregations"`
}

type esHits struct {
	Total esTotal  `json:"total"`
	Hits  []esHit  `json:"hits"`
}

type esTotal struct {
	Value    int64  `json:"value"`
	Relation string `json:"relation"`
}

type esHit struct {
	ID        string              `json:"_id"`
	Score     float64             `json:"_score"`
	Source    json.RawMessage     `json:"_source"`
	Highlight map[string][]string `json:"highlight,omitempty"`
}

type esAggregation struct {
	Buckets []esBucket `json:"buckets"`
}

type esBucket struct {
	Key      any   `json:"key"`
	DocCount int64 `json:"doc_count"`
}
