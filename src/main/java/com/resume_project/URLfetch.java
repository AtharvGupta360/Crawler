package com.resume_project;

import org.jsoup.Jsoup;
import org.jsoup.nodes.Document;
import org.jsoup.nodes.Element;
import org.jsoup.select.Elements;

import java.io.IOException;
import java.util.HashSet;
import java.util.Set;

/**
 * Fetches and extracts links from a given URL using Jsoup.
 */
public class URLfetch {

    public Set<String> fetchUrls(String url) throws IOException {

        Set<String> urls = new HashSet<>();

        Document doc = Jsoup.connect(url)
                .timeout(5000)
                .userAgent("Mozilla/5.0 (compatible; WebCrawler/1.0)")
                .followRedirects(true)
                .get();

        Elements links = doc.select("a[href]");

        for (Element link : links) {
            String absUrl = link.absUrl("href");

            // filter useless links
            if (absUrl.isEmpty() ||
                    absUrl.startsWith("mailto:") ||
                    absUrl.startsWith("javascript:") ||
                    absUrl.startsWith("tel:") ||
                    absUrl.contains("#")) {
                continue;
            }

            urls.add(absUrl);
        }

        return urls;
    }
}