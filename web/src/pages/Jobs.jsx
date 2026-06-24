import { useState } from 'react';
import { useApi } from '../hooks/useApi';
import { jobs, companies as companiesApi } from '../api/client';
import JobCard from '../components/JobCard';
import FilterBar from '../components/FilterBar';
import Pagination from '../components/Pagination';

export default function Jobs() {
  const [filters, setFilters] = useState({ limit: 20, offset: 0 });

  const { data, loading, refetch } = useApi(
    () => jobs.list(filters),
    [filters.seniority, filters.location_type, filters.company_id, filters.offset]
  );

  const { data: companiesData } = useApi(() => companiesApi.list(), []);

  const jobList = data?.jobs || [];
  const total = data?.total || 0;
  const companyList = companiesData?.companies || [];

  const handleFilterChange = (newFilters) => {
    setFilters({ ...filters, ...newFilters });
  };

  return (
    <div className="animate-fade-up">
      <div className="page-header">
        <div>
          <h1 className="page-title">Job Listings</h1>
          <p className="page-subtitle">{total.toLocaleString()} active jobs from {companyList.length} companies</p>
        </div>
      </div>

      <FilterBar
        filters={filters}
        onChange={handleFilterChange}
        companies={companyList}
      />

      {loading ? (
        <div className="job-list">
          {[1, 2, 3, 4, 5].map((i) => (
            <div key={i} className="skeleton skeleton-card" />
          ))}
        </div>
      ) : jobList.length > 0 ? (
        <>
          <div className="job-list">
            {jobList.map((job) => (
              <JobCard key={job.id} job={job} />
            ))}
          </div>
          <Pagination
            offset={filters.offset || 0}
            limit={filters.limit || 20}
            total={total}
            onChange={(newOffset) => setFilters({ ...filters, offset: newOffset })}
          />
        </>
      ) : (
        <div className="empty-state">
          <div className="empty-state-icon">🔍</div>
          <div className="empty-state-title">No jobs found</div>
          <div className="empty-state-text">
            Try adjusting your filters or trigger a crawl to collect new jobs.
          </div>
        </div>
      )}
    </div>
  );
}
