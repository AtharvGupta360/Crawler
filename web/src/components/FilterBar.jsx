export default function FilterBar({ filters, onChange, companies }) {
  const handleChange = (key, value) => {
    onChange({ ...filters, [key]: value, offset: 0 });
  };

  return (
    <div className="filter-bar">
      <select
        className="select"
        value={filters.seniority || ''}
        onChange={(e) => handleChange('seniority', e.target.value)}
      >
        <option value="">All Seniority</option>
        <option value="intern">Intern</option>
        <option value="junior">Junior</option>
        <option value="mid">Mid</option>
        <option value="senior">Senior</option>
        <option value="lead">Lead</option>
        <option value="staff">Staff</option>
      </select>

      <select
        className="select"
        value={filters.location_type || ''}
        onChange={(e) => handleChange('location_type', e.target.value)}
      >
        <option value="">All Locations</option>
        <option value="remote">Remote</option>
        <option value="hybrid">Hybrid</option>
        <option value="onsite">On-site</option>
      </select>

      {companies && companies.length > 0 && (
        <select
          className="select"
          value={filters.company_id || ''}
          onChange={(e) => handleChange('company_id', e.target.value)}
        >
          <option value="">All Companies</option>
          {companies.map((c) => (
            <option key={c.id} value={c.id}>{c.name}</option>
          ))}
        </select>
      )}

      {(filters.seniority || filters.location_type || filters.company_id) && (
        <button
          className="btn btn-ghost btn-sm"
          onClick={() => onChange({ limit: filters.limit, offset: 0 })}
        >
          Clear filters
        </button>
      )}
    </div>
  );
}
