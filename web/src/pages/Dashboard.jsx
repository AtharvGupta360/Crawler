import { useState, useEffect, useRef, useCallback } from 'react';
import { useApi } from '../hooks/useApi';
import { jobs, trends as trendsApi, crawl } from '../api/client';
import { useToast } from '../hooks/useToast';
import JobCard from '../components/JobCard';
import { Link } from 'react-router-dom';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';

// ── Crawl progress panel ──────────────────────────────────────────────────────

function statusIcon(s) {
  if (s === 'pending') return '⏸';
  if (s === 'running') return '🔄';
  if (s === 'done') return '✅';
  return '❌';
}

function CrawlProgress({ status }) {
  const done = status.completed_count ?? 0;
  const total = status.total_companies ?? 0;
  const pct = total > 0 ? Math.round((done / total) * 100) : 0;
  const isRunning = status.is_running;

  return (
    <div className="card" style={{
      marginBottom: '24px',
      borderLeft: `3px solid ${isRunning ? 'var(--accent)' : 'var(--success)'}`,
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px' }}>
        <h3 className="section-title" style={{ marginBottom: 0 }}>
          {isRunning ? '🔄 Crawling in progress…' : '✅ Crawl complete'}
        </h3>
        <span style={{ fontSize: '13px', color: 'var(--text-secondary)' }}>
          {done}/{total} companies
          {status.total_jobs_new > 0 && ` · +${status.total_jobs_new} new jobs`}
          {!isRunning && status.total_jobs_found > 0 && ` · ${status.total_jobs_found} found`}
        </span>
      </div>

      {/* Progress bar */}
      <div style={{ height: '4px', background: 'var(--border)', borderRadius: '2px', marginBottom: '16px' }}>
        <div style={{
          height: '100%',
          width: `${pct}%`,
          background: isRunning ? 'var(--accent)' : 'var(--success)',
          borderRadius: '2px',
          transition: 'width 0.4s ease',
        }} />
      </div>

      {/* Per-company status grid */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: '8px' }}>
        {(status.companies || []).map((c) => (
          <div key={c.slug} style={{
            display: 'flex', alignItems: 'flex-start', gap: '8px',
            padding: '8px 10px',
            background: 'var(--bg-secondary)',
            borderRadius: 'var(--radius-sm)',
            fontSize: '13px',
            opacity: c.status === 'pending' ? 0.5 : 1,
          }}>
            <span style={{ fontSize: '15px', lineHeight: 1.4 }}>{statusIcon(c.status)}</span>
            <div>
              <div style={{ fontWeight: 500, color: 'var(--text-primary)' }}>{c.name}</div>
              <div style={{ fontSize: '11px', marginTop: '2px' }}>
                {c.status === 'running' && (
                  <span style={{ color: 'var(--accent)' }}>Crawling…</span>
                )}
                {c.status === 'done' && (
                  <span style={{ color: 'var(--text-secondary)' }}>
                    {c.jobs_new > 0 ? `+${c.jobs_new} new` : `${c.jobs_found} found`}
                  </span>
                )}
                {c.status === 'failed' && (
                  <span style={{ color: 'var(--danger)' }} title={c.error}>Failed</span>
                )}
                {c.status === 'pending' && (
                  <span style={{ color: 'var(--text-tertiary)' }}>Waiting…</span>
                )}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

export default function Dashboard() {
  const toast = useToast();
  const [crawlStatus, setCrawlStatus] = useState(null);
  const [showProgress, setShowProgress] = useState(false);
  const pollRef = useRef(null);

  const { data: statsData, loading: statsLoading, refetch: refetchStats } = useApi(
    () => jobs.stats(), []
  );
  const { data: jobsData, loading: jobsLoading, refetch: refetchJobs } = useApi(
    () => jobs.list({ limit: 5, offset: 0 }), []
  );
  const { data: trendData } = useApi(
    () => trendsApi.skills({ days: 30, limit: 10 }), []
  );

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const fetchStatus = useCallback(async () => {
    try {
      const s = await crawl.status();
      setCrawlStatus(s);
      if (!s.is_running) {
        stopPolling();
        // Refresh stats and jobs once the crawl finishes
        refetchStats();
        refetchJobs();
      }
      return s;
    } catch {
      stopPolling();
      return null;
    }
  }, [stopPolling, refetchStats, refetchJobs]);

  const startPolling = useCallback(() => {
    if (pollRef.current) return;
    pollRef.current = setInterval(fetchStatus, 2000);
  }, [fetchStatus]);

  // On mount: check if a crawl is already running (server-side startup crawl)
  useEffect(() => {
    crawl.status().then((s) => {
      setCrawlStatus(s);
      if (s.is_running) {
        setShowProgress(true);
        startPolling();
      }
    }).catch(() => {});

    return stopPolling;
  }, [startPolling, stopPolling]);

  const handleTriggerCrawl = async () => {
    if (crawlStatus?.is_running) {
      toast.info('Already crawling', 'A crawl is already in progress.');
      return;
    }
    try {
      await crawl.triggerAll();
      setShowProgress(true);
      startPolling();
      // Fetch status immediately so the panel appears right away
      const s = await crawl.status();
      if (s) setCrawlStatus(s);
    } catch (err) {
      toast.error('Crawl failed', err.message);
    }
  };

  const stats = statsData || {};
  const recentJobs = jobsData?.jobs || [];

  // Detect if all visible jobs are from the demo seeder
  const allDemo = recentJobs.length > 0 &&
    recentJobs.every((j) => j.external_id?.startsWith('demo-'));

  // Build chart data
  const chartData = [];
  if (trendData?.trends) {
    const dateMap = {};
    trendData.trends.forEach((t) => {
      const date = t.snapshot_date?.split('T')[0];
      if (date) dateMap[date] = (dateMap[date] || 0) + t.job_count;
    });
    Object.entries(dateMap)
      .sort(([a], [b]) => a.localeCompare(b))
      .forEach(([date, count]) => {
        chartData.push({ date: date.slice(5), jobs: count });
      });
  }

  return (
    <div className="animate-fade-up">

      {/* Demo data notice */}
      {allDemo && !showProgress && (
        <div style={{
          display: 'flex', alignItems: 'center', gap: '10px',
          padding: '12px 16px', marginBottom: '20px',
          background: 'var(--warning-bg)',
          border: '1px solid hsla(35, 90%, 55%, 0.25)',
          borderRadius: 'var(--radius)',
          fontSize: '14px',
          color: 'var(--text-secondary)',
        }}>
          <span style={{ fontSize: '18px' }}>⚠️</span>
          <span>
            You are viewing <strong style={{ color: 'var(--warning)' }}>demo data</strong>.
            Click <strong>Trigger Crawl</strong> to fetch real-time job listings from Greenhouse, Lever, and Ashby.
          </span>
        </div>
      )}

      {/* Live crawl progress panel */}
      {showProgress && crawlStatus && (
        <CrawlProgress status={crawlStatus} />
      )}

      {/* Stats */}
      <div className="stat-grid">
        <div className="stat-card">
          <div className="stat-card-label">Active Jobs</div>
          <div className="stat-card-value">
            {statsLoading ? '—' : (stats.active_jobs || 0).toLocaleString()}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Companies</div>
          <div className="stat-card-value">
            {statsLoading ? '—' : (stats.companies || 0).toLocaleString()}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">New This Week</div>
          <div className="stat-card-value">
            {statsLoading ? '—' : (stats.new_this_week || 0).toLocaleString()}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Avg Salary</div>
          <div className="stat-card-value" style={{ color: 'var(--success)' }}>
            {statsLoading ? '—' : stats.avg_salary_max
              ? `$${Math.round(stats.avg_salary_max / 1000)}k`
              : 'N/A'
            }
          </div>
        </div>
      </div>

      {/* Chart + Quick Actions */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '20px', marginBottom: '24px' }}>
        {/* Trend Chart */}
        <div className="chart-container">
          <div className="chart-title">Skill Demand (30 days)</div>
          {chartData.length > 0 ? (
            <ResponsiveContainer width="100%" height={220}>
              <AreaChart data={chartData}>
                <defs>
                  <linearGradient id="colorJobs" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="hsl(250, 85%, 65%)" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="hsl(250, 85%, 65%)" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <XAxis
                  dataKey="date"
                  stroke="var(--text-tertiary)"
                  fontSize={11}
                  tickLine={false}
                  axisLine={false}
                />
                <YAxis
                  stroke="var(--text-tertiary)"
                  fontSize={11}
                  tickLine={false}
                  axisLine={false}
                />
                <Tooltip
                  contentStyle={{
                    background: 'var(--bg-secondary)',
                    border: '1px solid var(--border)',
                    borderRadius: '8px',
                    color: 'var(--text-primary)',
                    fontSize: '13px',
                  }}
                />
                <Area
                  type="monotone"
                  dataKey="jobs"
                  stroke="hsl(250, 85%, 65%)"
                  strokeWidth={2}
                  fill="url(#colorJobs)"
                />
              </AreaChart>
            </ResponsiveContainer>
          ) : (
            <div className="empty-state" style={{ padding: '40px 20px' }}>
              <div className="empty-state-text">No trend data yet. Run a crawl to get started.</div>
            </div>
          )}
        </div>

        {/* Quick Actions */}
        <div className="card" style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <h3 className="section-title">Quick Actions</h3>
          <button
            className="btn btn-primary"
            onClick={handleTriggerCrawl}
            disabled={crawlStatus?.is_running}
            style={{ opacity: crawlStatus?.is_running ? 0.6 : 1 }}
          >
            {crawlStatus?.is_running ? '⏳ Crawling…' : '🚀 Trigger Crawl'}
          </button>
          <Link to="/search" className="btn btn-secondary" style={{ textAlign: 'center' }}>
            🔍 Search Jobs
          </Link>
          <Link to="/match" className="btn btn-secondary" style={{ textAlign: 'center' }}>
            🎯 Match Resume
          </Link>
          <Link to="/trends" className="btn btn-secondary" style={{ textAlign: 'center' }}>
            📊 View Trends
          </Link>

          {/* Last crawl summary */}
          {!crawlStatus?.is_running && crawlStatus?.completed_at && (
            <div style={{
              marginTop: 'auto', padding: '10px 12px',
              background: 'var(--bg-secondary)', borderRadius: 'var(--radius-sm)',
              fontSize: '12px', color: 'var(--text-secondary)',
            }}>
              <div>Last crawl</div>
              <div style={{ color: 'var(--text-primary)', fontWeight: 500, marginTop: '2px' }}>
                +{crawlStatus.total_jobs_new} new · {crawlStatus.total_jobs_found} found
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Recent Jobs */}
      <div>
        <div className="page-header">
          <h2 className="section-title" style={{ marginBottom: 0 }}>Recent Jobs</h2>
          <Link to="/jobs" className="btn btn-ghost btn-sm">View all →</Link>
        </div>

        {jobsLoading ? (
          <div className="job-list">
            {[1, 2, 3].map((i) => (
              <div key={i} className="skeleton skeleton-card" />
            ))}
          </div>
        ) : recentJobs.length > 0 ? (
          <div className="job-list">
            {recentJobs.map((job) => (
              <JobCard key={job.id} job={job} />
            ))}
          </div>
        ) : (
          <div className="empty-state">
            <div className="empty-state-icon">💼</div>
            <div className="empty-state-title">No jobs yet</div>
            <div className="empty-state-text">
              Trigger a crawl to start collecting job postings.
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
