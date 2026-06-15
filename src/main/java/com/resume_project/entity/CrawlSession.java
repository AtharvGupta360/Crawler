package com.resume_project.entity;

import jakarta.persistence.*;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.List;

/**
 * Represents a single crawl session — one user-initiated crawl job.
 * Persisted to the database for history and analytics.
 */
@Entity
@Table(name = "crawl_sessions")
public class CrawlSession {

    public enum Status {
        RUNNING, COMPLETED, FAILED, STOPPED
    }

    @Id
    @GeneratedValue(strategy = GenerationType.UUID)
    private String id;

    @Column(nullable = false)
    private String startUrl;

    private int maxDepth;
    private int maxThreads;

    @Enumerated(EnumType.STRING)
    private Status status = Status.RUNNING;

    private LocalDateTime startTime;
    private LocalDateTime endTime;

    private int totalUrlsCrawled;
    private long durationMs;

    @OneToMany(mappedBy = "session", cascade = CascadeType.ALL, orphanRemoval = true)
    @OrderBy("discoveredAt ASC")
    private List<CrawlResult> results = new ArrayList<>();

    public CrawlSession() {}

    public CrawlSession(String startUrl, int maxDepth, int maxThreads) {
        this.startUrl = startUrl;
        this.maxDepth = maxDepth;
        this.maxThreads = maxThreads;
        this.startTime = LocalDateTime.now();
    }

    // --- Getters and Setters ---

    public String getId() { return id; }
    public void setId(String id) { this.id = id; }

    public String getStartUrl() { return startUrl; }
    public void setStartUrl(String startUrl) { this.startUrl = startUrl; }

    public int getMaxDepth() { return maxDepth; }
    public void setMaxDepth(int maxDepth) { this.maxDepth = maxDepth; }

    public int getMaxThreads() { return maxThreads; }
    public void setMaxThreads(int maxThreads) { this.maxThreads = maxThreads; }

    public Status getStatus() { return status; }
    public void setStatus(Status status) { this.status = status; }

    public LocalDateTime getStartTime() { return startTime; }
    public void setStartTime(LocalDateTime startTime) { this.startTime = startTime; }

    public LocalDateTime getEndTime() { return endTime; }
    public void setEndTime(LocalDateTime endTime) { this.endTime = endTime; }

    public int getTotalUrlsCrawled() { return totalUrlsCrawled; }
    public void setTotalUrlsCrawled(int totalUrlsCrawled) { this.totalUrlsCrawled = totalUrlsCrawled; }

    public long getDurationMs() { return durationMs; }
    public void setDurationMs(long durationMs) { this.durationMs = durationMs; }

    public List<CrawlResult> getResults() { return results; }
    public void setResults(List<CrawlResult> results) { this.results = results; }
}
