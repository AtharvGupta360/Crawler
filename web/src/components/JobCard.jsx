import { Link } from 'react-router-dom';

function formatSalary(min, max, currency = 'USD') {
  if (!min && !max) return null;
  const fmt = (n) => {
    if (n >= 1000) return `${Math.round(n / 1000)}k`;
    return n;
  };
  if (min && max) return `$${fmt(min)} – $${fmt(max)}`;
  if (min) return `$${fmt(min)}+`;
  return `Up to $${fmt(max)}`;
}

function timeAgo(dateStr) {
  if (!dateStr) return '';
  const diff = Date.now() - new Date(dateStr).getTime();
  const days = Math.floor(diff / 86400000);
  if (days === 0) return 'Today';
  if (days === 1) return 'Yesterday';
  if (days < 7) return `${days}d ago`;
  if (days < 30) return `${Math.floor(days / 7)}w ago`;
  return `${Math.floor(days / 30)}mo ago`;
}

export default function JobCard({ job }) {
  const salary = formatSalary(job.salary_min, job.salary_max, job.salary_currency);
  const skills = [
    ...(job.skills_required || []).slice(0, 3),
    ...(job.skills_preferred || []).slice(0, 2),
  ].slice(0, 5);
  const isDemo = job.external_id?.startsWith('demo-');

  return (
    <Link to={`/jobs/${job.id}`} className="job-card">
      <div className="job-card-body">
        <div className="job-card-header">
          <span className="job-card-title">{job.title}</span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            {isDemo && (
              <span style={{
                fontSize: '10px', fontWeight: 600, letterSpacing: '0.05em',
                padding: '2px 6px', borderRadius: '4px',
                background: 'var(--warning-bg)',
                color: 'var(--warning)',
                border: '1px solid hsla(35, 90%, 55%, 0.2)',
                textTransform: 'uppercase',
              }}>
                Demo
              </span>
            )}
            {salary && <span className="job-card-salary">{salary}</span>}
          </div>
        </div>

        <div className="job-card-company">
          {job.company?.name || 'Unknown Company'}
          {job.location && ` · ${job.location}`}
        </div>

        <div className="job-card-meta">
          {job.seniority_level && (
            <span className="badge badge-seniority">{job.seniority_level}</span>
          )}
          {job.location_type && (
            <span className={`badge ${job.location_type === 'remote' ? 'badge-remote' : 'badge-location'}`}>
              {job.location_type}
            </span>
          )}
          {job.employment_type && (
            <span className="badge badge-type">
              {job.employment_type.replace('_', ' ')}
            </span>
          )}
        </div>

        {skills.length > 0 && (
          <div className="job-card-skills">
            {skills.map((s, i) => (
              <span key={i} className="badge badge-skill">{s.name}</span>
            ))}
            {(job.skills_required?.length || 0) + (job.skills_preferred?.length || 0) > 5 && (
              <span className="badge badge-skill" style={{ opacity: 0.6 }}>
                +{(job.skills_required?.length || 0) + (job.skills_preferred?.length || 0) - 5}
              </span>
            )}
          </div>
        )}
      </div>

      <div className="job-card-actions">
        <span className="job-card-date">{timeAgo(job.first_seen_at)}</span>
      </div>
    </Link>
  );
}

export { formatSalary, timeAgo };
