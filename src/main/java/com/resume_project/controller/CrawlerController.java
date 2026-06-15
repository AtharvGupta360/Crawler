package com.resume_project.controller;

import com.resume_project.dto.CrawlRequest;
import com.resume_project.entity.CrawlResult;
import com.resume_project.entity.CrawlSession;
import com.resume_project.service.CrawlerService;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.servlet.mvc.method.annotation.SseEmitter;

import java.util.List;
import java.util.Map;

/**
 * REST API controller for the web crawler.
 * Provides endpoints to start, monitor, stream, and stop crawl sessions.
 */
@RestController
@RequestMapping("/api/crawl")
public class CrawlerController {

    private final CrawlerService crawlerService;

    public CrawlerController(CrawlerService crawlerService) {
        this.crawlerService = crawlerService;
    }

    /**
     * POST /api/crawl — Start a new crawl session.
     */
    @PostMapping
    public ResponseEntity<CrawlSession> startCrawl(@RequestBody CrawlRequest request) {
        if (request.getStartUrl() == null || request.getStartUrl().isBlank()) {
            return ResponseEntity.badRequest().build();
        }

        CrawlSession session = crawlerService.startCrawl(request);
        return ResponseEntity.ok(session);
    }

    /**
     * GET /api/crawl — Get all past crawl sessions.
     */
    @GetMapping
    public ResponseEntity<List<CrawlSession>> getAllSessions() {
        return ResponseEntity.ok(crawlerService.getAllSessions());
    }

    /**
     * GET /api/crawl/{sessionId} — Get a specific session's status and metadata.
     */
    @GetMapping("/{sessionId}")
    public ResponseEntity<CrawlSession> getSession(@PathVariable String sessionId) {
        return crawlerService.getSession(sessionId)
                .map(ResponseEntity::ok)
                .orElse(ResponseEntity.notFound().build());
    }

    /**
     * GET /api/crawl/{sessionId}/results — Get all crawled URLs for a session.
     */
    @GetMapping("/{sessionId}/results")
    public ResponseEntity<List<CrawlResult>> getResults(@PathVariable String sessionId) {
        return ResponseEntity.ok(crawlerService.getResults(sessionId));
    }

    /**
     * GET /api/crawl/{sessionId}/stream — Real-time SSE stream of crawl events.
     */
    @GetMapping(value = "/{sessionId}/stream", produces = MediaType.TEXT_EVENT_STREAM_VALUE)
    public SseEmitter streamCrawl(@PathVariable String sessionId) {
        return crawlerService.createEmitter(sessionId);
    }

    /**
     * POST /api/crawl/{sessionId}/stop — Stop an active crawl.
     */
    @PostMapping("/{sessionId}/stop")
    public ResponseEntity<Map<String, Object>> stopCrawl(@PathVariable String sessionId) {
        boolean stopped = crawlerService.stopCrawl(sessionId);
        return ResponseEntity.ok(Map.of(
                "sessionId", sessionId,
                "stopped", stopped,
                "message", stopped ? "Crawl stopped" : "No active crawl found"
        ));
    }
}
