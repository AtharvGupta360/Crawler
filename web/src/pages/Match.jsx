import { useState } from 'react';
import { match as matchApi } from '../api/client';
import { useAuth } from '../hooks/useAuth';
import { useToast } from '../hooks/useToast';
import TagsInput from '../components/TagsInput';
import { Link } from 'react-router-dom';
import { formatSalary, timeAgo } from '../components/JobCard';

export default function Match() {
  const { user } = useAuth();
  const toast = useToast();

  const [resumeText, setResumeText] = useState(user?.resume_text || '');
  const [knownSkills, setKnownSkills] = useState(user?.known_skills || []);
  const [targetRoles, setTargetRoles] = useState(user?.target_roles || []);
  const [targetSeniority, setTargetSeniority] = useState(user?.target_seniority || []);
  const [targetLocations, setTargetLocations] = useState(user?.target_locations || []);
  const [saveToProfile, setSaveToProfile] = useState(false);
  const [results, setResults] = useState(null);
  const [loading, setLoading] = useState(false);
  const [candidateCount, setCandidateCount] = useState(0);

  const handleMatch = async () => {
    if (!resumeText.trim() && knownSkills.length === 0) {
      toast.warning('Missing input', 'Please paste your resume or add some skills');
      return;
    }

    setLoading(true);
    try {
      const data = await matchApi.resume({
        resume_text: resumeText,
        known_skills: knownSkills,
        target_roles: targetRoles,
        target_seniority: targetSeniority,
        target_locations: targetLocations,
        save_to_profile: saveToProfile,
        limit: 20,
        candidate_limit: 500,
      });
      setResults(data.matches || []);
      setCandidateCount(data.candidates || 0);
      toast.success('Match complete', `Found ${data.total} matching jobs from ${data.candidates} candidates`);
    } catch (err) {
      toast.error('Match failed', err.message);
    } finally {
      setLoading(false);
    }
  };

  const getScoreClass = (score) => {
    if (score >= 70) return 'match-score-high';
    if (score >= 40) return 'match-score-medium';
    return 'match-score-low';
  };

  return (
    <div className="animate-fade-up">
      <div className="page-header">
        <div>
          <h1 className="page-title">Resume Match</h1>
          <p className="page-subtitle">Find jobs that match your skills and preferences</p>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '400px 1fr', gap: 24 }}>
        {/* Input Panel */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          <div className="card">
            <h3 className="section-title">Your Resume</h3>
            <textarea
              className="textarea"
              placeholder="Paste your resume text here…"
              value={resumeText}
              onChange={(e) => setResumeText(e.target.value)}
              style={{ minHeight: 180 }}
            />
          </div>

          <div className="card">
            <h3 className="section-title">Skills & Preferences</h3>

            <div className="input-group" style={{ marginBottom: 12 }}>
              <label>Known Skills</label>
              <TagsInput
                value={knownSkills}
                onChange={setKnownSkills}
                placeholder="e.g. Go, Python, React"
              />
            </div>

            <div className="input-group" style={{ marginBottom: 12 }}>
              <label>Target Roles</label>
              <TagsInput
                value={targetRoles}
                onChange={setTargetRoles}
                placeholder="e.g. Backend Engineer"
              />
            </div>

            <div className="input-group" style={{ marginBottom: 12 }}>
              <label>Target Seniority</label>
              <TagsInput
                value={targetSeniority}
                onChange={setTargetSeniority}
                placeholder="e.g. junior, mid, senior"
              />
            </div>

            <div className="input-group" style={{ marginBottom: 12 }}>
              <label>Target Locations</label>
              <TagsInput
                value={targetLocations}
                onChange={setTargetLocations}
                placeholder="e.g. remote, San Francisco"
              />
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 16 }}>
              <label className="toggle">
                <input
                  type="checkbox"
                  checked={saveToProfile}
                  onChange={(e) => setSaveToProfile(e.target.checked)}
                />
                <span className="toggle-slider" />
              </label>
              <span style={{ fontSize: 13, color: 'var(--text-secondary)' }}>
                Save to my profile
              </span>
            </div>

            <button
              className="btn btn-primary"
              onClick={handleMatch}
              disabled={loading}
              style={{ width: '100%' }}
            >
              {loading ? '🔄 Matching…' : '🎯 Find Matching Jobs'}
            </button>
          </div>
        </div>

        {/* Results */}
        <div>
          {results ? (
            <>
              <div style={{ fontSize: 14, color: 'var(--text-secondary)', marginBottom: 16 }}>
                <strong>{results.length}</strong> matches from {candidateCount.toLocaleString()} candidates
              </div>

              {results.length > 0 ? (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                  {results.map((r, i) => (
                    <div key={i} className="match-result" style={{ animationDelay: `${i * 50}ms` }}>
                      <div className={`match-score-ring ${getScoreClass(r.score)}`}>
                        {r.score}
                      </div>

                      <div className="match-details">
                        <Link
                          to={`/jobs/${r.job.id}`}
                          style={{ fontSize: 15, fontWeight: 600, color: 'var(--text-primary)' }}
                        >
                          {r.job.title}
                        </Link>
                        <div style={{ fontSize: 13, color: 'var(--text-secondary)', marginBottom: 4 }}>
                          {r.job.company?.name || 'Unknown'}
                          {r.job.location && ` · ${r.job.location}`}
                          {r.job.salary_min && ` · ${formatSalary(r.job.salary_min, r.job.salary_max)}`}
                        </div>

                        {/* Score breakdown */}
                        <div style={{ display: 'flex', gap: 12, fontSize: 12, color: 'var(--text-tertiary)', marginBottom: 6 }}>
                          <span>Skills: {r.skill_score}</span>
                          <span>Prefs: {r.preference_score}</span>
                          <span>Fresh: {r.freshness_score}</span>
                        </div>

                        {/* Matched skills */}
                        {r.matched_skills?.length > 0 && (
                          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginBottom: 6 }}>
                            {r.matched_skills.map((s, j) => (
                              <span key={j} className="badge badge-skill" style={{ fontSize: 10, padding: '2px 6px' }}>
                                ✓ {s}
                              </span>
                            ))}
                          </div>
                        )}

                        {/* Missing skills */}
                        {r.missing_required_skills?.length > 0 && (
                          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginBottom: 6 }}>
                            {r.missing_required_skills.map((s, j) => (
                              <span key={j} className="badge" style={{
                                fontSize: 10, padding: '2px 6px',
                                background: 'var(--danger-bg)', color: 'var(--danger)',
                                border: '1px solid hsla(0,75%,55%,0.15)',
                              }}>
                                ✕ {s}
                              </span>
                            ))}
                          </div>
                        )}

                        {/* Reasons */}
                        {r.reasons?.length > 0 && (
                          <div className="match-reasons">
                            {r.reasons.map((reason, j) => (
                              <span key={j} className="match-reason">{reason}</span>
                            ))}
                          </div>
                        )}
                      </div>

                      <div style={{ fontSize: 12, color: 'var(--text-tertiary)' }}>
                        {timeAgo(r.job.first_seen_at)}
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="empty-state">
                  <div className="empty-state-icon">🎯</div>
                  <div className="empty-state-title">No matches found</div>
                  <div className="empty-state-text">
                    Try adding more skills or broadening your preferences.
                  </div>
                </div>
              )}
            </>
          ) : (
            <div className="empty-state">
              <div className="empty-state-icon">🎯</div>
              <div className="empty-state-title">Ready to match</div>
              <div className="empty-state-text">
                Paste your resume or add your skills on the left, then click "Find Matching Jobs" to see personalized recommendations.
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
