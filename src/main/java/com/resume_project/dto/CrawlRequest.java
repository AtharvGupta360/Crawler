package com.resume_project.dto;

/**
 * Data Transfer Object for incoming crawl requests from the frontend.
 */
public class CrawlRequest {

    private String startUrl;
    private int maxDepth = 2;
    private int maxThreads = 4;

    public CrawlRequest() {}

    public CrawlRequest(String startUrl, int maxDepth, int maxThreads) {
        this.startUrl = startUrl;
        this.maxDepth = maxDepth;
        this.maxThreads = maxThreads;
    }

    public String getStartUrl() { return startUrl; }
    public void setStartUrl(String startUrl) { this.startUrl = startUrl; }

    public int getMaxDepth() { return maxDepth; }
    public void setMaxDepth(int maxDepth) { this.maxDepth = maxDepth; }

    public int getMaxThreads() { return maxThreads; }
    public void setMaxThreads(int maxThreads) { this.maxThreads = maxThreads; }
}
