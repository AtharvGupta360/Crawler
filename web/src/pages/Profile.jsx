import { useState } from 'react';
import { useAuth } from '../hooks/useAuth';
import { useToast } from '../hooks/useToast';
import { auth } from '../api/client';
import TagsInput from '../components/TagsInput';

export default function Profile() {
  const { user, updateUser } = useAuth();
  const toast = useToast();

  const [name, setName] = useState(user?.name || '');
  const [targetRoles, setTargetRoles] = useState(user?.target_roles || []);
  const [targetSeniority, setTargetSeniority] = useState(user?.target_seniority || []);
  const [targetLocations, setTargetLocations] = useState(user?.target_locations || []);
  const [knownSkills, setKnownSkills] = useState(user?.known_skills || []);
  const [learningSkills, setLearningSkills] = useState(user?.learning_skills || []);
  const [resumeText, setResumeText] = useState(user?.resume_text || '');
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      const updated = await auth.updateMe({
        name,
        target_roles: targetRoles,
        target_seniority: targetSeniority,
        target_locations: targetLocations,
        known_skills: knownSkills,
        learning_skills: learningSkills,
        resume_text: resumeText,
      });
      updateUser(updated);
      toast.success('Profile saved', 'Your preferences have been updated');
    } catch (err) {
      toast.error('Save failed', err.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="animate-fade-up" style={{ maxWidth: 800 }}>
      <div className="page-header">
        <div>
          <h1 className="page-title">My Profile</h1>
          <p className="page-subtitle">Manage your career preferences for better job matching</p>
        </div>
        <button className="btn btn-primary" onClick={handleSave} disabled={saving}>
          {saving ? 'Saving…' : 'Save Changes'}
        </button>
      </div>

      <div className="card" style={{ marginBottom: 16 }}>
        <h3 className="section-title">Basic Info</h3>
        <div className="profile-grid">
          <div className="input-group">
            <label>Name</label>
            <input
              className="input"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Your name"
            />
          </div>

          <div className="input-group">
            <label>Email</label>
            <input
              className="input"
              value={user?.email || ''}
              disabled
              style={{ opacity: 0.5 }}
            />
          </div>
        </div>
      </div>

      <div className="card" style={{ marginBottom: 16 }}>
        <h3 className="section-title">Career Preferences</h3>
        <div className="profile-grid">
          <div className="input-group">
            <label>Target Roles</label>
            <TagsInput
              value={targetRoles}
              onChange={setTargetRoles}
              placeholder="e.g. Backend Engineer, SDE"
            />
          </div>

          <div className="input-group">
            <label>Target Seniority</label>
            <TagsInput
              value={targetSeniority}
              onChange={setTargetSeniority}
              placeholder="e.g. junior, mid, senior"
            />
          </div>

          <div className="input-group profile-full">
            <label>Target Locations</label>
            <TagsInput
              value={targetLocations}
              onChange={setTargetLocations}
              placeholder="e.g. remote, San Francisco, New York"
            />
          </div>
        </div>
      </div>

      <div className="card" style={{ marginBottom: 16 }}>
        <h3 className="section-title">Skills</h3>
        <div className="profile-grid">
          <div className="input-group">
            <label>Known Skills</label>
            <TagsInput
              value={knownSkills}
              onChange={setKnownSkills}
              placeholder="e.g. Go, Python, PostgreSQL"
            />
          </div>

          <div className="input-group">
            <label>Learning Skills</label>
            <TagsInput
              value={learningSkills}
              onChange={setLearningSkills}
              placeholder="e.g. Kubernetes, gRPC"
            />
          </div>
        </div>
      </div>

      <div className="card">
        <h3 className="section-title">Resume</h3>
        <p style={{ fontSize: 13, color: 'var(--text-secondary)', marginBottom: 12 }}>
          Paste your resume text here. It's used for resume-job matching to find relevant positions.
        </p>
        <textarea
          className="textarea"
          value={resumeText}
          onChange={(e) => setResumeText(e.target.value)}
          placeholder="Paste your resume text here…"
          style={{ minHeight: 200 }}
        />
      </div>
    </div>
  );
}
