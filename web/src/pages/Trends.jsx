import { useState } from 'react';
import { useApi } from '../hooks/useApi';
import { trends as trendsApi } from '../api/client';
import {
  AreaChart, Area, BarChart, Bar, XAxis, YAxis, Tooltip,
  ResponsiveContainer, CartesianGrid, Legend
} from 'recharts';

const CHART_COLORS = [
  'hsl(250, 85%, 65%)',
  'hsl(200, 80%, 55%)',
  'hsl(145, 70%, 50%)',
  'hsl(35, 90%, 55%)',
  'hsl(0, 75%, 55%)',
  'hsl(280, 60%, 55%)',
  'hsl(170, 70%, 45%)',
  'hsl(320, 70%, 55%)',
];

const tooltipStyle = {
  background: 'var(--bg-secondary)',
  border: '1px solid var(--border)',
  borderRadius: '8px',
  color: 'var(--text-primary)',
  fontSize: '13px',
};

export default function Trends() {
  const [tab, setTab] = useState('skills');
  const [days, setDays] = useState(30);
  const [skillFilter, setSkillFilter] = useState('');

  const { data: skillsData, loading: skillsLoading } = useApi(
    () => trendsApi.skills({ days, limit: 50, skill: skillFilter || undefined }),
    [days, skillFilter]
  );

  const { data: companiesData, loading: companiesLoading } = useApi(
    () => trendsApi.companies({ days, limit: 20 }),
    [days]
  );

  const { data: salariesData, loading: salariesLoading } = useApi(
    () => trendsApi.salaries({ limit: 20 }),
    []
  );

  // Process skills data into chart format
  const skillChartData = [];
  const skillNames = new Set();
  if (skillsData?.trends) {
    const dateMap = {};
    skillsData.trends.forEach((t) => {
      const date = t.snapshot_date?.split('T')[0];
      if (date) {
        if (!dateMap[date]) dateMap[date] = {};
        dateMap[date][t.skill_name] = t.job_count;
        skillNames.add(t.skill_name);
      }
    });
    Object.entries(dateMap)
      .sort(([a], [b]) => a.localeCompare(b))
      .forEach(([date, skills]) => {
        skillChartData.push({ date: date.slice(5), ...skills });
      });
  }
  const topSkills = [...skillNames].slice(0, 8);

  // Process company data
  const companyChartData = companiesData?.companies || [];

  // Process salary data
  const salaryChartData = (salariesData?.salaries || []).slice(0, 15).map((s) => ({
    name: s.skill_name,
    min: s.avg_salary_min || 0,
    max: s.avg_salary_max || 0,
  }));

  return (
    <div className="animate-fade-up">
      <div className="page-header">
        <div>
          <h1 className="page-title">Trend Analytics</h1>
          <p className="page-subtitle">Market insights from crawled job data</p>
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          <select
            className="select"
            value={days}
            onChange={(e) => setDays(Number(e.target.value))}
            style={{ width: 'auto' }}
          >
            <option value={7}>7 days</option>
            <option value={14}>14 days</option>
            <option value={30}>30 days</option>
            <option value={90}>90 days</option>
          </select>
        </div>
      </div>

      {/* Tabs */}
      <div className="tabs">
        <button className={`tab ${tab === 'skills' ? 'active' : ''}`} onClick={() => setTab('skills')}>
          Skill Demand
        </button>
        <button className={`tab ${tab === 'companies' ? 'active' : ''}`} onClick={() => setTab('companies')}>
          Company Hiring
        </button>
        <button className={`tab ${tab === 'salaries' ? 'active' : ''}`} onClick={() => setTab('salaries')}>
          Salary Ranges
        </button>
      </div>

      {/* Skills Tab */}
      {tab === 'skills' && (
        <div>
          <div style={{ marginBottom: 16 }}>
            <input
              className="input"
              placeholder="Filter by skill name (e.g. Go, React, Python)…"
              value={skillFilter}
              onChange={(e) => setSkillFilter(e.target.value)}
              style={{ maxWidth: 400 }}
            />
          </div>

          <div className="chart-container">
            <div className="chart-title">Skill Demand Over Time</div>
            {skillsLoading ? (
              <div className="skeleton" style={{ height: 300 }} />
            ) : skillChartData.length > 0 ? (
              <ResponsiveContainer width="100%" height={350}>
                <AreaChart data={skillChartData}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                  <XAxis dataKey="date" stroke="var(--text-tertiary)" fontSize={11} tickLine={false} />
                  <YAxis stroke="var(--text-tertiary)" fontSize={11} tickLine={false} />
                  <Tooltip contentStyle={tooltipStyle} />
                  <Legend />
                  {topSkills.map((skill, i) => (
                    <Area
                      key={skill}
                      type="monotone"
                      dataKey={skill}
                      stroke={CHART_COLORS[i % CHART_COLORS.length]}
                      fill={CHART_COLORS[i % CHART_COLORS.length]}
                      fillOpacity={0.1}
                      strokeWidth={2}
                    />
                  ))}
                </AreaChart>
              </ResponsiveContainer>
            ) : (
              <div className="empty-state" style={{ padding: 40 }}>
                <div className="empty-state-text">No skill trend data available. Refresh trends from the backend.</div>
              </div>
            )}
          </div>

          {/* Top Skills Table */}
          {skillsData?.trends && skillsData.trends.length > 0 && (
            <div className="card" style={{ marginTop: 20 }}>
              <h3 className="section-title">Top Skills by Job Count</h3>
              <div style={{ overflowX: 'auto' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border)' }}>
                      <th style={{ textAlign: 'left', padding: '10px 12px', color: 'var(--text-secondary)', fontWeight: 600 }}>Skill</th>
                      <th style={{ textAlign: 'right', padding: '10px 12px', color: 'var(--text-secondary)', fontWeight: 600 }}>Jobs</th>
                      <th style={{ textAlign: 'right', padding: '10px 12px', color: 'var(--text-secondary)', fontWeight: 600 }}>Avg Salary</th>
                    </tr>
                  </thead>
                  <tbody>
                    {[...new Map(skillsData.trends.map(t => [t.skill_name, t])).values()]
                      .sort((a, b) => b.job_count - a.job_count)
                      .slice(0, 20)
                      .map((t, i) => (
                        <tr key={i} style={{ borderBottom: '1px solid var(--border)' }}>
                          <td style={{ padding: '10px 12px' }}>
                            <span className="badge badge-skill">{t.skill_name}</span>
                          </td>
                          <td style={{ padding: '10px 12px', textAlign: 'right' }}>{t.job_count}</td>
                          <td style={{ padding: '10px 12px', textAlign: 'right', color: 'var(--success)' }}>
                            {t.avg_salary_min && t.avg_salary_max
                              ? `$${Math.round(t.avg_salary_min / 1000)}k – $${Math.round(t.avg_salary_max / 1000)}k`
                              : '—'
                            }
                          </td>
                        </tr>
                      ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Companies Tab */}
      {tab === 'companies' && (
        <div className="chart-container">
          <div className="chart-title">Most Active Hiring Companies</div>
          {companiesLoading ? (
            <div className="skeleton" style={{ height: 300 }} />
          ) : companyChartData.length > 0 ? (
            <ResponsiveContainer width="100%" height={400}>
              <BarChart data={companyChartData} layout="vertical" margin={{ left: 100 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                <XAxis type="number" stroke="var(--text-tertiary)" fontSize={11} />
                <YAxis
                  dataKey="name"
                  type="category"
                  stroke="var(--text-tertiary)"
                  fontSize={12}
                  width={90}
                />
                <Tooltip contentStyle={tooltipStyle} />
                <Bar dataKey="count" fill="hsl(250, 85%, 65%)" radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <div className="empty-state" style={{ padding: 40 }}>
              <div className="empty-state-text">No company hiring data available.</div>
            </div>
          )}
        </div>
      )}

      {/* Salaries Tab */}
      {tab === 'salaries' && (
        <div className="chart-container">
          <div className="chart-title">Average Salary Ranges by Skill</div>
          {salariesLoading ? (
            <div className="skeleton" style={{ height: 300 }} />
          ) : salaryChartData.length > 0 ? (
            <ResponsiveContainer width="100%" height={400}>
              <BarChart data={salaryChartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" />
                <XAxis dataKey="name" stroke="var(--text-tertiary)" fontSize={11} angle={-30} textAnchor="end" height={60} />
                <YAxis stroke="var(--text-tertiary)" fontSize={11} tickFormatter={(v) => `$${v / 1000}k`} />
                <Tooltip contentStyle={tooltipStyle} formatter={(v) => `$${(v / 1000).toFixed(0)}k`} />
                <Legend />
                <Bar dataKey="min" name="Avg Min" fill="hsl(200, 80%, 55%)" radius={[4, 4, 0, 0]} />
                <Bar dataKey="max" name="Avg Max" fill="hsl(145, 70%, 50%)" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <div className="empty-state" style={{ padding: 40 }}>
              <div className="empty-state-text">No salary data available.</div>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
