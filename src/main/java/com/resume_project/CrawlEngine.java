package com.resume_project;

import com.resume_project.entity.CrawlResult;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Phaser;
import java.util.function.BiConsumer;

/**
 * Core crawl engine — manages the thread pool and phaser-based coordination.
 * This replaces the old static WebCrawler class. It is fully decoupled from I/O
 * and receives a callback for each crawled URL.
 */
public class CrawlEngine {

    private final URLfetch urlfetch;
    private final URLStore urlStore;
    private final int maxDepth;
    private final Phaser phaser;
    private final ExecutorService executor;
    private final BiConsumer<CrawlResult, Integer> resultCallback;
    private volatile boolean stopped = false;

    public CrawlEngine(String startUrl, int maxDepth, int maxThreads,
                       BiConsumer<CrawlResult, Integer> resultCallback) {
        this.urlfetch = new URLfetch();
        this.urlStore = new URLStore();
        this.maxDepth = maxDepth;
        this.phaser = new Phaser(1); // register self
        this.executor = Executors.newFixedThreadPool(maxThreads);
        this.resultCallback = resultCallback;

        // Seed the starting URL
        urlStore.addUrl(startUrl, 0, null);
    }

    /**
     * Starts the crawl. This method blocks until all tasks are done.
     */
    public void start() {
        submitTask();
        phaser.arriveAndAwaitAdvance();
        executor.shutdown();
    }

    /**
     * Submits a new crawl task to the thread pool.
     */
    public void submitTask() {
        if (stopped) return;
        phaser.register();
        executor.submit(new Crawler(urlfetch, urlStore, maxDepth, phaser, resultCallback, this));
    }

    /**
     * Stops the crawl engine. Running tasks will finish, but no new tasks are submitted.
     */
    public void stop() {
        stopped = true;
        executor.shutdownNow();
    }

    public boolean isStopped() {
        return stopped;
    }

    public int getVisitedCount() {
        return urlStore.getVisitedCount();
    }
}
