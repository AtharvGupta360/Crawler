package com.resume_project;

import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.BlockingQueue;
import java.util.concurrent.LinkedBlockingQueue;

/**
 * Thread-safe URL store that tracks visited URLs and maintains a work queue.
 * Used internally by the crawl engine; results are persisted separately via the service layer.
 */
public class URLStore {

    public static class UrlDepthPair {
        public final String url;
        public final int depth;
        public final String parentUrl;

        public UrlDepthPair(String url, int depth, String parentUrl) {
            this.url = url;
            this.depth = depth;
            this.parentUrl = parentUrl;
        }
    }

    private final ConcurrentHashMap<String, Boolean> visitedUrl = new ConcurrentHashMap<>();
    private final BlockingQueue<UrlDepthPair> urlQueue = new LinkedBlockingQueue<>();

    /**
     * Adds a URL to the queue if it hasn't been visited before.
     * @return true if the URL was new and added, false if already visited.
     */
    public boolean addUrl(String url, int depth, String parentUrl) {
        if (visitedUrl.putIfAbsent(url, true) == null) {
            urlQueue.offer(new UrlDepthPair(url, depth, parentUrl));
            return true;
        }
        return false;
    }

    /**
     * Backward-compatible overload (no parent URL).
     */
    public boolean addUrl(String url, int depth) {
        return addUrl(url, depth, null);
    }

    public UrlDepthPair getNextUrl() {
        return urlQueue.poll();
    }

    public boolean isQueueEmpty() {
        return urlQueue.isEmpty();
    }

    public int getVisitedCount() {
        return visitedUrl.size();
    }

    public Set<String> getVisitedUrls() {
        return visitedUrl.keySet();
    }
}
