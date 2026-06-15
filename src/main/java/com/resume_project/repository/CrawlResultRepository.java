package com.resume_project.repository;

import com.resume_project.entity.CrawlResult;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface CrawlResultRepository extends JpaRepository<CrawlResult, Long> {

    /**
     * Find all results belonging to a specific session, ordered by discovery time.
     */
    List<CrawlResult> findBySessionIdOrderByDiscoveredAtAsc(String sessionId);

    /**
     * Count results for a session.
     */
    long countBySessionId(String sessionId);
}
