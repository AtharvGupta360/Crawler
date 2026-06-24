import { useParams, Link } from 'react-router-dom';
import { useApi } from '../hooks/useApi';
import { jobs } from '../api/client';
import { formatSalary, timeAgo } from '../components/JobCard';

export default function JobDetail() {
  const { id } = useParams();
  const { data: job, loading, error } = useApi(() => jobs.get(id), [id]);

  if (loading) {
    return (
      <div className="animate-fade-up job-detail">
        <div className="skeleton skeleton-title" style={{ width: '70%', height: 32 }} />
        <div className="skeleton skeleton-text" style={{ width: '40%' }} />
        <div style={{ height: 16 }} />
        <div className="skeleton" style={{ height: 400, borderRadius: 'var(--radius-lg)' }} />
      </div>
    );
  }

  if (error || !job) {
    return (
      <div className="empty-state">
        <div className="empty-state-icon">😕</div>
        <div className="empty-state-title">Job not found</div>
        <div className="empty-state-text">{error || 'This job may have been removed.'}</div>
        <Link to="/jobs" className="btn btn-primary" style={{ marginTop: 16 }}>
          Back to Jobs
        </Link>
      </div>
    );
  }

  const salary = formatSalary(job.salary_min, job.salary_max, job.salary_currency);
  const requiredSkills = job.skills_required || [];
  const preferredSkills = job.skills_preferred || [];

  return (
    <div className="animate-fade-up job-detail">
      <div style={{ marginBottom: 16 }}>
        <Link to="/jobs" className="btn btn-ghost btn-sm">← Back to Jobs</Link>
      </div>

      <div className="job-detail-header">
        <h1 className="job-detail-title">{job.title}</h1>
        <div className="job-detail-company">
          {job.company?.name || 'Unknown Company'}
          {job.location && ` · ${job.location}`}
        </div>

        <div className="job-detail-meta">
          {job.seniority_level && (
            <span className="badge badge-seniority">{job.seniority_level}</span>
          )}
          {job.location_type && (
            <span className={`badge ${job.location_type === 'remote' ? 'badge-remote' : 'badge-location'}`}>
              {job.location_type}
            </span>
          )}
          {job.employment_type && (
            <span className="badge badge-type">{job.employment_type.replace('_', ' ')}</span>
          )}
          {salary && (
            <span className="badge" style={{ background: 'var(--success-bg)', color: 'var(--success)', border: '1px solid hsla(145,70%,50%,0.15)' }}>
              {salary}
            </span>
          )}
          <span style={{ fontSize: 12, color: 'var(--text-tertiary)' }}>
            Posted {timeAgo(job.first_seen_at)}
          </span>
        </div>

        <a href={job.apply_url} target="_blank" rel="noopener noreferrer" className="btn btn-primary">
          Apply Now →
        </a>
      </div>

      <div className="job-detail-body">
        {/* Main Content */}
        <div>
          {/* AI Summary */}
          {job.ai_summary && (
            <div className="card" style={{ marginBottom: 20, borderLeft: '3px solid var(--accent)' }}>
              <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--accent)', marginBottom: 8 }}>
                ✨ AI Summary
              </div>
              <div style={{ fontSize: 14, lineHeight: 1.7, color: 'var(--text-secondary)' }}>
                {job.ai_summary}
              </div>
            </div>
          )}

          {/* Description */}
          <div className="card">
            <h3 className="section-title">Job Description</h3>
            <div
              className="job-detail-description"
              dangerouslySetInnerHTML={{
                __html: job.description_raw || '<p>No description available.</p>',
              }}
            />
          </div>
        </div>

        {/* Sidebar */}
        <div className="job-detail-sidebar">
          {/* Skills */}
          {(requiredSkills.length > 0 || preferredSkills.length > 0) && (
            <div className="card">
              <h3 className="section-title">Skills</h3>

              {requiredSkills.length > 0 && (
                <div style={{ marginBottom: 12 }}>
                  <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-tertiary)', marginBottom: 8 }}>
                    Required
                  </div>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                    {requiredSkills.map((s, i) => (
                      <span key={i} className="badge badge-skill">{s.name}</span>
                    ))}
                  </div>
                </div>
              )}

              {preferredSkills.length > 0 && (
                <div>
                  <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--text-tertiary)', marginBottom: 8 }}>
                    Preferred
                  </div>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                    {preferredSkills.map((s, i) => (
                      <span key={i} className="badge badge-skill" style={{ opacity: 0.7 }}>{s.name}</span>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Details */}
          <div className="card">
            <h3 className="section-title">Details</h3>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 10, fontSize: 13 }}>
              {job.department && (
                <div>
                  <span style={{ color: 'var(--text-tertiary)' }}>Department: </span>
                  <span>{job.department}</span>
                </div>
              )}
              {job.team && (
                <div>
                  <span style={{ color: 'var(--text-tertiary)' }}>Team: </span>
                  <span>{job.team}</span>
                </div>
              )}
              {(job.experience_years_min || job.experience_years_max) && (
                <div>
                  <span style={{ color: 'var(--text-tertiary)' }}>Experience: </span>
                  <span>
                    {job.experience_years_min && job.experience_years_max
                      ? `${job.experience_years_min}–${job.experience_years_max} years`
                      : job.experience_years_min
                        ? `${job.experience_years_min}+ years`
                        : `Up to ${job.experience_years_max} years`
                    }
                  </span>
                </div>
              )}
              {job.education_level && (
                <div>
                  <span style={{ color: 'var(--text-tertiary)' }}>Education: </span>
                  <span>{job.education_level}</span>
                </div>
              )}
              <div>
                <span style={{ color: 'var(--text-tertiary)' }}>First seen: </span>
                <span>{new Date(job.first_seen_at).toLocaleDateString()}</span>
              </div>
              <div>
                <span style={{ color: 'var(--text-tertiary)' }}>Last updated: </span>
                <span>{new Date(job.last_seen_at).toLocaleDateString()}</span>
              </div>
            </div>
          </div>

          {/* Company */}
          {job.company && (
            <div className="card">
              <h3 className="section-title">Company</h3>
              <div style={{ fontSize: 15, fontWeight: 600, marginBottom: 6 }}>
                {job.company.name}
              </div>
              {job.company.industry && (
                <div style={{ fontSize: 13, color: 'var(--text-secondary)', marginBottom: 4 }}>
                  {job.company.industry}
                </div>
              )}
              {job.company.size_range && (
                <div style={{ fontSize: 13, color: 'var(--text-secondary)', marginBottom: 4 }}>
                  {job.company.size_range} employees
                </div>
              )}
              {job.company.website && (
                <a href={job.company.website} target="_blank" rel="noopener noreferrer" style={{ fontSize: 13 }}>
                  Visit website →
                </a>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
