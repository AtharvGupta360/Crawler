package com.resume_project.entity;

import com.fasterxml.jackson.annotation.JsonIgnore;
import jakarta.persistence.*;
import java.time.LocalDateTime;

/**
 * Represents a single crawled URL result within a crawl session.
 */
@Entity
@Table(name = "crawl_results")
public class CrawlResult {

    public enum CrawlStatus {
        SUCCESS, FAILED, SKIPPED
    }

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(nullable = false, length = 2048)
    private String url;

    private int depth;

    private String parentUrl;

    private int discoveredLinksCount;

    @Enumerated(EnumType.STRING)
    private CrawlStatus crawlStatus = CrawlStatus.SUCCESS;

    private String errorMessage;

    private LocalDateTime discoveredAt;

    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "session_id")
    @JsonIgnore
    private CrawlSession session;

    public CrawlResult() {}

    public CrawlResult(String url, int depth, String parentUrl, CrawlSession session) {
        this.url = url;
        this.depth = depth;
        this.parentUrl = parentUrl;
        this.session = session;
        this.discoveredAt = LocalDateTime.now();
    }

    // --- Getters and Setters ---

    public Long getId() { return id; }
    public void setId(Long id) { this.id = id; }

    public String getUrl() { return url; }
    public void setUrl(String url) { this.url = url; }

    public int getDepth() { return depth; }
    public void setDepth(int depth) { this.depth = depth; }

    public String getParentUrl() { return parentUrl; }
    public void setParentUrl(String parentUrl) { this.parentUrl = parentUrl; }

    public int getDiscoveredLinksCount() { return discoveredLinksCount; }
    public void setDiscoveredLinksCount(int discoveredLinksCount) { this.discoveredLinksCount = discoveredLinksCount; }

    public CrawlStatus getCrawlStatus() { return crawlStatus; }
    public void setCrawlStatus(CrawlStatus crawlStatus) { this.crawlStatus = crawlStatus; }

    public String getErrorMessage() { return errorMessage; }
    public void setErrorMessage(String errorMessage) { this.errorMessage = errorMessage; }

    public LocalDateTime getDiscoveredAt() { return discoveredAt; }
    public void setDiscoveredAt(LocalDateTime discoveredAt) { this.discoveredAt = discoveredAt; }

    public CrawlSession getSession() { return session; }
    public void setSession(CrawlSession session) { this.session = session; }
}
