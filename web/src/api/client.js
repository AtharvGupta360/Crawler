const BASE = '/api/v1';

function getToken() {
  return localStorage.getItem('jc_token');
}

function setToken(token) {
  localStorage.setItem('jc_token', token);
}

function clearToken() {
  localStorage.removeItem('jc_token');
}

async function request(method, path, { body, params } = {}) {
  const url = new URL(path.startsWith('http') ? path : `${BASE}${path}`, window.location.origin);
  if (params) {
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined && v !== null && v !== '') url.searchParams.set(k, v);
    });
  }

  const headers = { 'Content-Type': 'application/json' };
  const token = getToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch(url.toString(), {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });

  if (res.status === 401) {
    clearToken();
    if (!path.includes('/auth/')) {
      window.location.href = '/login';
    }
  }

  const data = await res.json().catch(() => ({}));

  if (!res.ok) {
    const err = new Error(data.error || `Request failed: ${res.status}`);
    err.status = res.status;
    err.data = data;
    throw err;
  }

  return data;
}

// ── Auth ──
export const auth = {
  register: (email, password, name) =>
    request('POST', '/auth/register', { body: { email, password, name } }),
  login: (email, password) =>
    request('POST', '/auth/login', { body: { email, password } }),
  getMe: () => request('GET', '/auth/me'),
  updateMe: (data) => request('PUT', '/auth/me', { body: data }),
};

// ── Jobs ──
export const jobs = {
  list: (params) => request('GET', '/jobs', { params }),
  get: (id) => request('GET', `/jobs/${id}`),
  stats: () => request('GET', '/jobs/stats'),
};

// ── Search ──
export const search = {
  query: (params) => request('GET', '/search', { params }),
};

// ── Companies ──
export const companies = {
  list: () => request('GET', '/companies'),
  create: (data) => request('POST', '/companies', { body: data }),
};

// ── Crawl ──
export const crawl = {
  triggerAll: () => request('POST', '/crawl/trigger'),
  triggerCompany: (slug) => request('POST', `/crawl/trigger/${slug}`),
  runs: (params) => request('GET', '/crawl/runs', { params }),
  health: () => request('GET', '/crawl/health'),
};

// ── Alerts ──
export const alerts = {
  list: () => request('GET', '/alerts'),
  create: (data) => request('POST', '/alerts', { body: data }),
  update: (id, data) => request('PUT', `/alerts/${id}`, { body: data }),
  delete: (id) => request('DELETE', `/alerts/${id}`),
};

// ── Notifications ──
export const notifications = {
  list: (params) => request('GET', '/notifications', { params }),
  markRead: (id) => request('PUT', `/notifications/${id}/read`),
  markAllRead: () => request('PUT', '/notifications/read-all'),
};

// ── Match ──
export const match = {
  resume: (data) => request('POST', '/match/resume', { body: data }),
  recommendations: (params) => request('GET', '/match/recommendations', { params }),
};

// ── Trends ──
export const trends = {
  skills: (params) => request('GET', '/trends/skills', { params }),
  companies: (params) => request('GET', '/trends/companies', { params }),
  salaries: (params) => request('GET', '/trends/salaries', { params }),
  refresh: (params) => request('POST', '/trends/refresh', { params }),
};

// ── Health ──
export const health = {
  check: () => fetch('/health').then((r) => r.json()),
};

export { getToken, setToken, clearToken };
