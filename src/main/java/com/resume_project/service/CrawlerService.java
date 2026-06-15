package com.resume_project.service;

import com.resume_project.CrawlEngine;
import com.resume_project.dto.CrawlRequest;
import com.resume_project.entity.CrawlResult;
import com.resume_project.entity.CrawlSession;
import com.resume_project.repository.CrawlResultRepository;
import com.resume_project.repository.CrawlSessionRepository;
import org.springframework.stereotype.Service;
import org.springframework.web.servlet.mvc.method.annotation.SseEmitter;

import java.io.IOException;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Service layer that orchestrates crawl sessions.
 * Manages the lifecycle (start, monitor, stop) and persists results to the database.
 */
@Service
public class CrawlerService {

    private final CrawlSessionRepository sessionRepository;
    private final CrawlResultRepository resultRepository;

    // Active crawl engines, keyed by session ID
    private final Map<String, CrawlEngine> activeEngines = new ConcurrentHashMap<>();

    // SSE emitters for real-time streaming, keyed by session ID
    private final Map<String, List<SseEmitter>> emitters = new ConcurrentHashMap<>();

    // Thread pool for running crawl jobs asynchronously
    private final ExecutorService crawlExecutor = Executors.newCachedThreadPool();

    public CrawlerService(CrawlSessionRepository sessionRepository,
                          CrawlResultRepository resultRepository) {
        this.sessionRepository = sessionRepository;
        this.resultRepository = resultRepository;
    }

    /**
     * Starts a new crawl session asynchronously.
     * Returns the session immediately; crawling happens in the background.
     */
    public CrawlSession startCrawl(CrawlRequest request) {
        // Create and persist the session
        CrawlSession session = new CrawlSession(
                request.getStartUrl(),
                request.getMaxDepth(),
                request.getMaxThreads()
        );
        sessionRepository.save(session);

        String sessionId = session.getId();
        long startTimeMs = System.currentTimeMillis();

        // Create the crawl engine with a result callback
        CrawlEngine engine = new CrawlEngine(
                request.getStartUrl(),
                request.getMaxDepth(),
                request.getMaxThreads(),
                (result, visitedCount) -> onCrawlResult(sessionId, result, visitedCount)
        );

        activeEngines.put(sessionId, engine);

        // Run the crawl in a background thread
        crawlExecutor.submit(() -> {
            try {
                engine.start(); // blocks until crawl is done

                // Crawl completed
                long durationMs = System.currentTimeMillis() - startTimeMs;
                CrawlSession completed = sessionRepository.findById(sessionId).orElse(null);
                if (completed != null) {
                    completed.setStatus(engine.isStopped()
                            ? CrawlSession.Status.STOPPED
                            : CrawlSession.Status.COMPLETED);
                    completed.setEndTime(LocalDateTime.now());
                    completed.setDurationMs(durationMs);
                    completed.setTotalUrlsCrawled(engine.getVisitedCount());
                    sessionRepository.save(completed);
                }

                // Notify SSE clients that crawl is done
                sendSseEvent(sessionId, "complete", Map.of(
                        "sessionId", sessionId,
                        "status", completed != null ? completed.getStatus().name() : "COMPLETED",
                        "totalUrls", engine.getVisitedCount(),
                        "durationMs", durationMs
                ));

            } catch (Exception e) {
                CrawlSession failed = sessionRepository.findById(sessionId).orElse(null);
                if (failed != null) {
                    failed.setStatus(CrawlSession.Status.FAILED);
                    failed.setEndTime(LocalDateTime.now());
                    sessionRepository.save(failed);
                }
                sendSseEvent(sessionId, "error", Map.of("message", e.getMessage()));
            } finally {
                activeEngines.remove(sessionId);
                cleanupEmitters(sessionId);
            }
        });

        return session;
    }

    /**
     * Callback invoked by the crawl engine for each URL processed.
     * Persists the result and streams it via SSE.
     */
    private void onCrawlResult(String sessionId, CrawlResult result, int visitedCount) {
        // Link to the session and persist
        CrawlSession session = sessionRepository.findById(sessionId).orElse(null);
        if (session == null) return;

        result.setSession(session);
        result.setDiscoveredAt(LocalDateTime.now());
        resultRepository.save(result);

        // Stream to connected SSE clients
        sendSseEvent(sessionId, "crawl-result", Map.of(
                "url", result.getUrl(),
                "depth", result.getDepth(),
                "parentUrl", result.getParentUrl() != null ? result.getParentUrl() : "",
                "status", result.getCrawlStatus().name(),
                "linksFound", result.getDiscoveredLinksCount(),
                "error", result.getErrorMessage() != null ? result.getErrorMessage() : "",
                "totalVisited", visitedCount
        ));
    }

    /**
     * Creates an SSE emitter for real-time streaming of crawl events.
     */
    public SseEmitter createEmitter(String sessionId) {
        SseEmitter emitter = new SseEmitter(0L); // no timeout
        emitters.computeIfAbsent(sessionId, k -> new java.util.concurrent.CopyOnWriteArrayList<>())
                .add(emitter);

        emitter.onCompletion(() -> removeEmitter(sessionId, emitter));
        emitter.onTimeout(() -> removeEmitter(sessionId, emitter));
        emitter.onError(e -> removeEmitter(sessionId, emitter));

        return emitter;
    }

    private void sendSseEvent(String sessionId, String eventName, Object data) {
        List<SseEmitter> sessionEmitters = emitters.get(sessionId);
        if (sessionEmitters == null) return;

        for (SseEmitter emitter : sessionEmitters) {
            try {
                emitter.send(SseEmitter.event()
                        .name(eventName)
                        .data(data));
            } catch (IOException e) {
                removeEmitter(sessionId, emitter);
            }
        }
    }

    private void removeEmitter(String sessionId, SseEmitter emitter) {
        List<SseEmitter> sessionEmitters = emitters.get(sessionId);
        if (sessionEmitters != null) {
            sessionEmitters.remove(emitter);
        }
    }

    private void cleanupEmitters(String sessionId) {
        List<SseEmitter> sessionEmitters = emitters.remove(sessionId);
        if (sessionEmitters != null) {
            for (SseEmitter emitter : sessionEmitters) {
                try { emitter.complete(); } catch (Exception ignored) {}
            }
        }
    }

    /**
     * Gets a crawl session by ID.
     */
    public Optional<CrawlSession> getSession(String sessionId) {
        return sessionRepository.findById(sessionId);
    }

    /**
     * Gets all results for a session.
     */
    public List<CrawlResult> getResults(String sessionId) {
        return resultRepository.findBySessionIdOrderByDiscoveredAtAsc(sessionId);
    }

    /**
     * Gets all past sessions (most recent first).
     */
    public List<CrawlSession> getAllSessions() {
        return sessionRepository.findAllByOrderByStartTimeDesc();
    }

    /**
     * Stops an active crawl session.
     */
    public boolean stopCrawl(String sessionId) {
        CrawlEngine engine = activeEngines.get(sessionId);
        if (engine != null) {
            engine.stop();
            return true;
        }
        return false;
    }

    /**
     * Checks if a session is actively crawling.
     */
    public boolean isActive(String sessionId) {
        return activeEngines.containsKey(sessionId);
    }
}
