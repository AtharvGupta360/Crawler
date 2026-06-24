import { useState } from 'react';
import { search as searchApi } from '../api/client';
import JobCard from '../components/JobCard';
import Pagination from '../components/Pagination';

export default function Search() {
  const [query, setQuery] = useState('');
  const [filters, setFilters] = useState({});
  const [results, setResults] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [offset, setOffset] = useState(0);
  const limit = 20;

  const doSearch = async (newOffset = 0) => {
    if (!query.trim()) return;
    setLoading(true);
    setError('');
    try {
      const data = await searchApi.query({
        q: query,
        limit,
        offset: newOffset,
        ...filters,
      });
      setResults(data);
      setOffset(newOffset);
    } catch (err) {
      setError(err.message || 'Search failed');
    } finally {
      setLoading(false);
    }
  };

  const handleSubmit = (e) => {
    e.preventDefault();
    doSearch(0);
  };

  const facets = results?.facets || {};
  const jobs = results?.results || [];
  const total = results?.total || 0;

  return (
    <div className="animate-fade-up">
      <div className="page-header">
        <div>
          <h1 className="page-title">Search Jobs</h1>
          <p className="page-subtitle">Full-text search across all job postings</p>
        </div>
      </div>

      <form onSubmit={handleSubmit} style={{ marginBottom: 24 }}>
        <div className="search-container" style={{ maxWidth: '100%' }}>
          <span className="search-icon">🔍</span>
          <input
            className="search-input"
            type="text"
            placeholder="Search by title, skills, company, description…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            autoFocus
          />
        </div>
      </form>

      {error && (
        <div className="auth-error" style={{ marginBottom: 16 }}>
          {error}. Search requires Elasticsearch to be running.
        </div>
      )}

      {results && (
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 260px', gap: 20 }}>
          {/* Results */}
          <div>
            <div style={{ fontSize: 14, color: 'var(--text-secondary)', marginBottom: 16 }}>
              {total.toLocaleString()} results for "<strong>{results.query}</strong>"
            </div>

            {loading ? (
              <div className="job-list">
                {[1, 2, 3].map((i) => (
                  <div key={i} className="skeleton skeleton-card" />
                ))}
              </div>
            ) : jobs.length > 0 ? (
              <>
                <div className="job-list">
                  {jobs.map((job) => (
                    <JobCard key={job.id || job.job_id} job={job} />
                  ))}
                </div>
                <Pagination
                  offset={offset}
                  limit={limit}
                  total={total}
                  onChange={(o) => doSearch(o)}
                />
              </>
            ) : (
              <div className="empty-state">
                <div className="empty-state-icon">🔍</div>
                <div className="empty-state-title">No results</div>
                <div className="empty-state-text">
                  Try different keywords or broader search terms.
                </div>
              </div>
            )}
          </div>

          {/* Facets Sidebar */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
            {facets.seniority_levels && Object.keys(facets.seniority_levels).length > 0 && (
              <div className="card">
                <h3 style={{ fontSize: 13, fontWeight: 600, marginBottom: 10, color: 'var(--text-secondary)' }}>
                  Seniority Level
                </h3>
                {Object.entries(facets.seniority_levels).map(([level, count]) => (
                  <div
                    key={level}
                    className="nav-link"
                    style={{ padding: '6px 8px', fontSize: 13, justifyContent: 'space-between' }}
                    onClick={() => {
                      setFilters({ ...filters, seniority: level });
                      doSearch(0);
                    }}
                  >
                    <span>{level}</span>
                    <span style={{ color: 'var(--text-tertiary)', fontSize: 12 }}>{count}</span>
                  </div>
                ))}
              </div>
            )}

            {facets.location_types && Object.keys(facets.location_types).length > 0 && (
              <div className="card">
                <h3 style={{ fontSize: 13, fontWeight: 600, marginBottom: 10, color: 'var(--text-secondary)' }}>
                  Location Type
                </h3>
                {Object.entries(facets.location_types).map(([type, count]) => (
                  <div
                    key={type}
                    className="nav-link"
                    style={{ padding: '6px 8px', fontSize: 13, justifyContent: 'space-between' }}
                    onClick={() => {
                      setFilters({ ...filters, location_type: type });
                      doSearch(0);
                    }}
                  >
                    <span>{type}</span>
                    <span style={{ color: 'var(--text-tertiary)', fontSize: 12 }}>{count}</span>
                  </div>
                ))}
              </div>
            )}

            {facets.companies && Object.keys(facets.companies).length > 0 && (
              <div className="card">
                <h3 style={{ fontSize: 13, fontWeight: 600, marginBottom: 10, color: 'var(--text-secondary)' }}>
                  Companies
                </h3>
                {Object.entries(facets.companies).slice(0, 10).map(([company, count]) => (
                  <div
                    key={company}
                    className="nav-link"
                    style={{ padding: '6px 8px', fontSize: 13, justifyContent: 'space-between' }}
                    onClick={() => {
                      setFilters({ ...filters, company });
                      doSearch(0);
                    }}
                  >
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{company}</span>
                    <span style={{ color: 'var(--text-tertiary)', fontSize: 12 }}>{count}</span>
                  </div>
                ))}
              </div>
            )}

            {Object.keys(filters).length > 0 && (
              <button
                className="btn btn-ghost btn-sm"
                onClick={() => {
                  setFilters({});
                  doSearch(0);
                }}
              >
                Clear all filters
              </button>
            )}
          </div>
        </div>
      )}

      {!results && !loading && (
        <div className="empty-state">
          <div className="empty-state-icon">🔍</div>
          <div className="empty-state-title">Start searching</div>
          <div className="empty-state-text">
            Enter keywords to search across job titles, descriptions, skills, and companies.
          </div>
        </div>
      )}
    </div>
  );
}
