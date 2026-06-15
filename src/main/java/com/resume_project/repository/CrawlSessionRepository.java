package com.resume_project.repository;

import com.resume_project.entity.CrawlSession;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface CrawlSessionRepository extends JpaRepository<CrawlSession, String> {

    /**
     * Find all sessions ordered by start time descending (most recent first).
     */
    List<CrawlSession> findAllByOrderByStartTimeDesc();
}
