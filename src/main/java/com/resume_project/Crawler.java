package com.resume_project;

import com.resume_project.entity.CrawlResult;

import java.util.Set;
import java.util.concurrent.Phaser;
import java.util.function.BiConsumer;

/**
 * A single crawl task that processes one URL from the queue.
 * Emits structured CrawlResult objects via a callback instead of printing to stdout.
 */
public class Crawler implements Runnable {

    private final URLfetch urlfetch;
    private final URLStore urlStore;
    private final int maxDepth;
    private final Phaser phaser;
    private final BiConsumer<CrawlResult, Integer> resultCallback;
    private final CrawlEngine engine;

    public Crawler(URLfetch urlfetch, URLStore urlStore, int maxDepth,
                   Phaser phaser, BiConsumer<CrawlResult, Integer> resultCallback,
                   CrawlEngine engine) {
        this.urlfetch = urlfetch;
        this.urlStore = urlStore;
        this.maxDepth = maxDepth;
        this.phaser = phaser;
        this.resultCallback = resultCallback;
        this.engine = engine;
    }

    @Override
    public void run() {
        try {
            URLStore.UrlDepthPair pair = urlStore.getNextUrl();
            if (pair == null) {
                return;
            }

            String url = pair.url;
            int depth = pair.depth;
            String parentUrl = pair.parentUrl;

            CrawlResult result = new CrawlResult();
            result.setUrl(url);
            result.setDepth(depth);
            result.setParentUrl(parentUrl);

            try {
                if (depth < maxDepth) {
                    Set<String> discoveredUrls = urlfetch.fetchUrls(url);
                    result.setDiscoveredLinksCount(discoveredUrls.size());
                    result.setCrawlStatus(CrawlResult.CrawlStatus.SUCCESS);

                    for (String newUrl : discoveredUrls) {
                        if (urlStore.addUrl(newUrl, depth + 1, url)) {
                            engine.submitTask();
                        }
                    }
                } else {
                    // At max depth — we visited but didn't crawl children
                    result.setDiscoveredLinksCount(0);
                    result.setCrawlStatus(CrawlResult.CrawlStatus.SUCCESS);
                }
            } catch (Exception e) {
                result.setCrawlStatus(CrawlResult.CrawlStatus.FAILED);
                result.setErrorMessage(e.getMessage());
            }

            // Emit the result via callback
            if (resultCallback != null) {
                resultCallback.accept(result, urlStore.getVisitedCount());
            }

        } catch (Exception e) {
            e.printStackTrace();
        } finally {
            phaser.arriveAndDeregister();
        }
    }
}
