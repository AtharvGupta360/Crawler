import { useApi } from '../hooks/useApi';
import { jobs, trends as trendsApi, crawl } from '../api/client';
import { useToast } from '../hooks/useToast';
import JobCard from '../components/JobCard';
import { Link } from 'react-router-dom';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';

export default function Dashboard() {
  const toast = useToast();

  const { data: statsData, loading: statsLoading } = useApi(() => jobs.stats(), []);
  const { data: jobsData, loading: jobsLoading } = useApi(
    () => jobs.list({ limit: 5, offset: 0 }),
    []
  );
  const { data: trendData } = useApi(
    () => trendsApi.skills({ days: 30, limit: 10 }),
    []
  );

  const stats = statsData || {};
  const recentJobs = jobsData?.jobs || [];

  // Build a simple chart from trend data (aggregate job_count per date)
  const chartData = [];
  if (trendData?.trends) {
    const dateMap = {};
    trendData.trends.forEach((t) => {
      const date = t.snapshot_date?.split('T')[0];
      if (date) {
        dateMap[date] = (dateMap[date] || 0) + t.job_count;
      }
    });
    Object.entries(dateMap)
      .sort(([a], [b]) => a.localeCompare(b))
      .forEach(([date, count]) => {
        chartData.push({ date: date.slice(5), jobs: count });
      });
  }

  const handleTriggerCrawl = async () => {
    try {
      await crawl.triggerAll();
      toast.success('Crawl triggered', 'Crawling all companies in the background');
    } catch (err) {
      toast.error('Crawl failed', err.message);
    }
  };

  return (
    <div className="animate-fade-up">
      {/* Stats */}
      <div className="stat-grid">
        <div className="stat-card">
          <div className="stat-card-label">Active Jobs</div>
          <div className="stat-card-value">
            {statsLoading ? '—' : (stats.total_active || 0).toLocaleString()}
          </div>
        </div>
        <div className="stat-card">
          <div className="stat-card-label">Companies</div>
          <div className="stat-card-value">
            {statsLoading ? '—' : (stats.total_companies || 0).toLocaleString()}
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

      {/* Chart + Recent Jobs */}
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
          <button className="btn btn-primary" onClick={handleTriggerCrawl}>
            🚀 Trigger Crawl
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
