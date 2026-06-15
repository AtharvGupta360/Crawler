/**
 * Web Crawler Dashboard — Frontend Application
 * Handles form submission, SSE streaming, real-time stats, and history.
 */

// ============================================================
// State
// ============================================================
const state = {
    currentSessionId: null,
    eventSource: null,
    stats: { urls: 0, maxDepth: 0, success: 0, failed: 0 },
    results: [],
    crawlStartTime: null,
};

// ============================================================
// DOM References
// ============================================================
const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => document.querySelectorAll(sel);

const els = {
    form: $('#crawl-form'),
    inputUrl: $('#input-url'),
    inputDepth: $('#input-depth'),
    inputThreads: $('#input-threads'),
    depthValue: $('#depth-value'),
    threadsValue: $('#threads-value'),
    btnStart: $('#btn-start'),
    btnStop: $('#btn-stop'),
    btnHistory: $('#btn-history'),
    btnBack: $('#btn-back'),

    heroSection: $('#hero-section'),
    dashboardSection: $('#dashboard-section'),
    historySection: $('#history-section'),

    statusDot: $('#status-dot'),
    statusText: $('#status-text'),
    sessionUrl: $('#session-url'),

    statUrls: $('#stat-urls'),
    statDepth: $('#stat-depth'),
    statSuccess: $('#stat-success'),
    statFailed: $('#stat-failed'),

    resultsBody: $('#results-body'),
    resultsCount: $('#results-count'),
    resultsEmpty: $('#results-empty'),

    historyList: $('#history-list'),
    historyEmpty: $('#history-empty'),

    toastContainer: $('#toast-container'),
};

// ============================================================
// Initialization
// ============================================================
document.addEventListener('DOMContentLoaded', () => {
    // Slider live values
    els.inputDepth.addEventListener('input', () => {
        els.depthValue.textContent = els.inputDepth.value;
    });
    els.inputThreads.addEventListener('input', () => {
        els.threadsValue.textContent = els.inputThreads.value;
    });

    // Form submission
    els.form.addEventListener('submit', handleStartCrawl);

    // Stop button
    els.btnStop.addEventListener('click', handleStopCrawl);

    // History button
    els.btnHistory.addEventListener('click', showHistory);

    // Back button
    els.btnBack.addEventListener('click', showHome);
});

// ============================================================
// Start Crawl
// ============================================================
async function handleStartCrawl(e) {
    e.preventDefault();

    const url = els.inputUrl.value.trim();
    if (!url) return;

    els.btnStart.disabled = true;
    els.btnStart.innerHTML = `
        <svg class="spin" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>
        Starting...
    `;

    try {
        const response = await fetch('/api/crawl', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                startUrl: url,
                maxDepth: parseInt(els.inputDepth.value),
                maxThreads: parseInt(els.inputThreads.value),
            }),
        });

        if (!response.ok) {
            throw new Error(`Server returned ${response.status}`);
        }

        const session = await response.json();
        state.currentSessionId = session.id;
        state.crawlStartTime = Date.now();
        resetStats();

        // Show dashboard
        showDashboard(url);

        // Connect to SSE stream
        connectSSE(session.id);

        showToast('Crawl started!', 'success');

    } catch (err) {
        showToast(`Failed to start crawl: ${err.message}`, 'error');
    } finally {
        els.btnStart.disabled = false;
        els.btnStart.innerHTML = `
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="5 3 19 12 5 21 5 3"/></svg>
            Start Crawling
        `;
    }
}

// ============================================================
// Stop Crawl
// ============================================================
async function handleStopCrawl() {
    if (!state.currentSessionId) return;

    try {
        await fetch(`/api/crawl/${state.currentSessionId}/stop`, { method: 'POST' });
        showToast('Crawl stopped.', 'info');
    } catch (err) {
        showToast(`Failed to stop: ${err.message}`, 'error');
    }
}

// ============================================================
// SSE — Real-time Event Stream
// ============================================================
function connectSSE(sessionId) {
    // Close any existing connection
    if (state.eventSource) {
        state.eventSource.close();
    }

    const eventSource = new EventSource(`/api/crawl/${sessionId}/stream`);
    state.eventSource = eventSource;

    eventSource.addEventListener('crawl-result', (e) => {
        const data = JSON.parse(e.data);
        onCrawlResult(data);
    });

    eventSource.addEventListener('complete', (e) => {
        const data = JSON.parse(e.data);
        onCrawlComplete(data);
        eventSource.close();
    });

    eventSource.addEventListener('error', (e) => {
        // SSE error could mean crawl is done and stream closed
        if (eventSource.readyState === EventSource.CLOSED) {
            return;
        }
        try {
            const data = JSON.parse(e.data);
            showToast(`Crawl error: ${data.message}`, 'error');
        } catch {
            // connection closed normally
        }
        eventSource.close();
    });
}

// ============================================================
// Event Handlers
// ============================================================
function onCrawlResult(data) {
    // Update stats
    state.stats.urls = data.totalVisited || (state.stats.urls + 1);

    if (data.depth > state.stats.maxDepth) {
        state.stats.maxDepth = data.depth;
    }

    if (data.status === 'SUCCESS') {
        state.stats.success++;
    } else {
        state.stats.failed++;
    }

    updateStatsUI();

    // Add result to table
    addResultRow(data);

    // Hide empty state
    els.resultsEmpty.classList.add('hidden');
}

function onCrawlComplete(data) {
    // Update status indicator
    const status = data.status || 'COMPLETED';
    els.statusDot.className = `pulse-dot ${status.toLowerCase()}`;
    els.statusText.textContent = status === 'STOPPED' ? 'Stopped' :
                                  status === 'FAILED' ? 'Failed' : 'Completed';

    // Update final stats
    if (data.totalUrls) state.stats.urls = data.totalUrls;
    updateStatsUI();

    // Hide stop button
    els.btnStop.classList.add('hidden');

    const durationSec = data.durationMs ? (data.durationMs / 1000).toFixed(1) : '—';
    showToast(`Crawl ${status.toLowerCase()} — ${data.totalUrls} URLs in ${durationSec}s`, 'success');
}

// ============================================================
// UI Updates
// ============================================================
function resetStats() {
    state.stats = { urls: 0, maxDepth: 0, success: 0, failed: 0 };
    state.results = [];
    els.resultsBody.innerHTML = '';
    els.resultsEmpty.classList.remove('hidden');
    updateStatsUI();
}

function updateStatsUI() {
    animateValue(els.statUrls, state.stats.urls);
    animateValue(els.statDepth, state.stats.maxDepth);
    animateValue(els.statSuccess, state.stats.success);
    animateValue(els.statFailed, state.stats.failed);

    els.resultsCount.textContent = `${state.stats.urls} results`;
}

function animateValue(el, target) {
    el.textContent = target;
}

function addResultRow(data) {
    const tbody = els.resultsBody;
    const row = document.createElement('tr');

    const statusClass = data.status === 'SUCCESS' ? 'success' : 'failed';
    const statusLabel = data.status === 'SUCCESS' ? '✓ OK' : '✗ Fail';

    const now = new Date();
    const timeStr = now.toLocaleTimeString('en-US', { hour12: false });

    row.innerHTML = `
        <td><span class="status-badge status-badge--${statusClass}">${statusLabel}</span></td>
        <td class="url-cell"><a href="${escapeHtml(data.url)}" target="_blank" rel="noopener">${escapeHtml(truncateUrl(data.url))}</a></td>
        <td><span class="depth-badge">${data.depth}</span></td>
        <td>${data.linksFound || 0}</td>
        <td class="time-cell">${timeStr}</td>
    `;

    // Prepend (newest first)
    if (tbody.firstChild) {
        tbody.insertBefore(row, tbody.firstChild);
    } else {
        tbody.appendChild(row);
    }

    // Limit displayed rows to 200 for performance
    while (tbody.children.length > 200) {
        tbody.removeChild(tbody.lastChild);
    }
}

// ============================================================
// Navigation
// ============================================================
function showDashboard(url) {
    els.heroSection.classList.add('hidden');
    els.historySection.classList.add('hidden');
    els.dashboardSection.classList.remove('hidden');

    els.statusDot.className = 'pulse-dot';
    els.statusText.textContent = 'Crawling...';
    els.sessionUrl.textContent = url;
    els.btnStop.classList.remove('hidden');
}

function showHome() {
    els.heroSection.classList.remove('hidden');
    els.dashboardSection.classList.add('hidden');
    els.historySection.classList.add('hidden');

    if (state.eventSource) {
        state.eventSource.close();
        state.eventSource = null;
    }
}

async function showHistory() {
    els.heroSection.classList.add('hidden');
    els.dashboardSection.classList.add('hidden');
    els.historySection.classList.remove('hidden');

    try {
        const response = await fetch('/api/crawl');
        const sessions = await response.json();
        renderHistory(sessions);
    } catch (err) {
        showToast(`Failed to load history: ${err.message}`, 'error');
    }
}

function renderHistory(sessions) {
    const list = els.historyList;
    list.innerHTML = '';

    if (!sessions || sessions.length === 0) {
        els.historyEmpty.classList.remove('hidden');
        return;
    }

    els.historyEmpty.classList.add('hidden');

    sessions.forEach((session) => {
        const card = document.createElement('div');
        card.className = 'history-card glass-panel';

        const statusColor = session.status === 'COMPLETED' ? 'var(--accent-green)' :
                            session.status === 'FAILED' ? 'var(--accent-red)' :
                            session.status === 'STOPPED' ? 'var(--accent-amber)' :
                            'var(--accent-cyan)';

        const dateStr = session.startTime
            ? new Date(session.startTime).toLocaleDateString('en-US', {
                  month: 'short', day: 'numeric', year: 'numeric',
                  hour: '2-digit', minute: '2-digit'
              })
            : '—';

        const durationStr = session.durationMs
            ? `${(session.durationMs / 1000).toFixed(1)}s`
            : '—';

        card.innerHTML = `
            <div class="history-card__info">
                <div class="history-card__url">${escapeHtml(session.startUrl)}</div>
                <div class="history-card__meta">
                    <span style="color: ${statusColor}; font-weight: 600;">${session.status}</span>
                    <span>${dateStr}</span>
                    <span>Depth: ${session.maxDepth}</span>
                    <span>Threads: ${session.maxThreads}</span>
                </div>
            </div>
            <div class="history-card__stats">
                <div class="history-card__stat">
                    <span class="history-card__stat-value">${session.totalUrlsCrawled || 0}</span>
                    <span class="history-card__stat-label">URLs</span>
                </div>
                <div class="history-card__stat">
                    <span class="history-card__stat-value">${durationStr}</span>
                    <span class="history-card__stat-label">Duration</span>
                </div>
            </div>
        `;

        card.addEventListener('click', () => viewSessionResults(session.id, session.startUrl));
        list.appendChild(card);
    });
}

async function viewSessionResults(sessionId, startUrl) {
    try {
        const response = await fetch(`/api/crawl/${sessionId}/results`);
        const results = await response.json();

        // Show dashboard with historical results
        resetStats();
        showDashboard(startUrl);
        els.btnStop.classList.add('hidden');
        els.statusDot.className = 'pulse-dot completed';
        els.statusText.textContent = 'Completed (History)';

        results.forEach((r) => {
            const data = {
                url: r.url,
                depth: r.depth,
                status: r.crawlStatus,
                linksFound: r.discoveredLinksCount,
                totalVisited: results.length,
            };
            onCrawlResult(data);
        });

    } catch (err) {
        showToast(`Failed to load results: ${err.message}`, 'error');
    }
}

// ============================================================
// Toast Notifications
// ============================================================
function showToast(message, type = 'info') {
    const toast = document.createElement('div');
    toast.className = `toast toast--${type}`;
    toast.textContent = message;

    els.toastContainer.appendChild(toast);

    setTimeout(() => {
        toast.style.opacity = '0';
        toast.style.transform = 'translateX(20px)';
        toast.style.transition = 'all 0.3s ease';
        setTimeout(() => toast.remove(), 300);
    }, 4000);
}

// ============================================================
// Utilities
// ============================================================
function escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

function truncateUrl(url) {
    if (!url) return '';
    if (url.length <= 70) return url;
    return url.substring(0, 67) + '...';
}
