'use strict';

// ── Theme ─────────────────────────────────────────────────────────────────

function applyTheme(theme) {
  document.documentElement.setAttribute('data-theme', theme);
  localStorage.setItem('sd-theme', theme);
}

function toggleTheme() {
  const current = document.documentElement.getAttribute('data-theme') || 'dark';
  applyTheme(current === 'dark' ? 'light' : 'dark');
}

document.getElementById('theme-toggle')?.addEventListener('click', toggleTheme);
document.getElementById('mob-theme-toggle')?.addEventListener('click', toggleTheme);

// ── API helpers ────────────────────────────────────────────────────────────

async function api(method, path, body) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body !== undefined) opts.body = JSON.stringify(body);
  const res = await fetch('/api/v1' + path, opts);
  if (res.status === 204) return null;
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

const apiGet   = p     => api('GET',    p);
const apiPost  = (p,b) => api('POST',   p, b);
const apiPut   = (p,b) => api('PUT',    p, b);
const apiDel   = p     => api('DELETE', p);
const apiPatch = (p,b) => api('PATCH',  p, b);

// ── Safe DOM helpers ───────────────────────────────────────────────────────

/** Create an element with optional class and text content. */
function el(tag, className, text) {
  const e = document.createElement(tag);
  if (className) e.className = className;
  if (text !== undefined) e.textContent = text;
  return e;
}

/** Append multiple children to a parent. */
function append(parent, ...children) {
  for (const c of children) parent.appendChild(c);
  return parent;
}

/** Set element text safely. */
function setText(id, text) {
  const e = document.getElementById(id);
  if (e) e.textContent = text ?? '';
}

// Approximate collection duration in seconds (matches backend batch constants).
const COLLECT_SECS = 6;

/** Build a status badge element. collectingSince is a Unix timestamp (seconds). */
function statusBadgeEl(status, collectingSince) {
  const map = {
    online:     ['badge-success', 'Online'],
    offline:    ['badge-danger',  'Offline'],
    unknown:    ['badge-neutral', 'Unknown'],
    error:      ['badge-danger',  'Error'],
  };
  if (status === 'collecting') {
    const badge = el('span', 'badge badge-collecting');
    if (collectingSince) badge.dataset.since = collectingSince;
    badge.textContent = collectingLabel(collectingSince);
    return badge;
  }
  const [cls, label] = map[status] || map.unknown;
  return el('span', `badge ${cls}`, label);
}

function collectingLabel(since) {
  if (!since) return 'Collecting…';
  const elapsed = Math.floor(Date.now() / 1000) - since;
  const remaining = Math.max(0, COLLECT_SECS - elapsed);
  return remaining > 0 ? `Collecting ~${remaining}s` : 'Finalizing…';
}

/** Build a port status dot + text span. */
function portStatusEl(status) {
  const wrap = el('span');
  const dot = el('span', 'port-status-dot ' + (
    status === 'up' ? 'dot-up' : status === 'disabled' ? 'dot-disabled' : 'dot-down'
  ));
  append(wrap, dot, document.createTextNode(status || '—'));
  return wrap;
}

// ── Toasts ────────────────────────────────────────────────────────────────

const toastContainer = (() => {
  const c = el('div', 'toast-container');
  document.body.appendChild(c);
  return c;
})();

function toast(msg, type = 'success') {
  const t = el('div', `toast toast-${type}`, msg);
  toastContainer.appendChild(t);
  setTimeout(() => t.remove(), 3500);
}

// ── Nav active state ──────────────────────────────────────────────────────

function setActiveNav() {
  const path = location.pathname;
  document.querySelectorAll('.nav-link, .mobile-nav-item').forEach(a => {
    const href = a.getAttribute('href');
    const active = href === '/' ? path === '/' : path.startsWith(href);
    a.classList.toggle('active', active);
  });
}
setActiveNav();


// ── Format helpers ────────────────────────────────────────────────────────

function fmtBytes(n) {
  if (n == null) return '—';
  if (n >= 1e9) return (n / 1e9).toFixed(2) + ' GB';
  if (n >= 1e6) return (n / 1e6).toFixed(2) + ' MB';
  if (n >= 1e3) return (n / 1e3).toFixed(1) + ' KB';
  return n + ' B';
}

function fmtNum(n) {
  if (n == null) return '—';
  return n.toLocaleString();
}

function formatDuration(ns) {
  if (!ns) return null;
  const secs = Math.floor(ns / 1e9);
  const d = Math.floor(secs / 86400);
  const h = Math.floor((secs % 86400) / 3600);
  const m = Math.floor((secs % 3600) / 60);
  const parts = [];
  if (d) parts.push(d + 'd');
  if (h) parts.push(h + 'h');
  if (m) parts.push(m + 'm');
  return parts.join(' ') || '<1m';
}

// ══════════════════════════════════════════════════════════════════════════
// DASHBOARD
// ══════════════════════════════════════════════════════════════════════════

const switchesGrid = document.getElementById('switches-grid');
if (switchesGrid) initDashboard();

let _pollTimer = null;
let _tickTimer = null;

function startCountdownTick() {
  clearInterval(_tickTimer);
  _tickTimer = setInterval(() => {
    document.querySelectorAll('.badge-collecting[data-since]').forEach(badge => {
      badge.textContent = collectingLabel(parseInt(badge.dataset.since));
    });
  }, 1000);
}

function stopCountdownTick() {
  clearInterval(_tickTimer);
  _tickTimer = null;
}

async function initDashboard() {
  await loadSwitches();
}

function scheduleCollectingPoll() {
  clearTimeout(_pollTimer);
  _pollTimer = setTimeout(async () => {
    const switches = await apiGet('/switches').catch(() => null);
    if (!switches) return;
    const anyCollecting = switches.some(s => s.status === 'collecting');
    renderSwitchGrid(switches);
    if (anyCollecting) {
      scheduleCollectingPoll();
    } else {
      stopCountdownTick();
    }
  }, 2000);
}

async function loadSwitches() {
  switchesGrid.textContent = '';
  const loading = el('p', 'text-muted', 'Loading switches…');
  loading.style.padding = '24px';
  switchesGrid.appendChild(loading);
  try {
    const switches = await apiGet('/switches');
    renderSwitchGrid(switches);
    if (switches && switches.some(s => s.status === 'collecting')) {
      startCountdownTick();
      scheduleCollectingPoll();
    }
  } catch (e) {
    switchesGrid.textContent = '';
    const err = el('p', 'alert alert-danger', 'Failed to load switches: ' + e.message);
    switchesGrid.appendChild(err);
  }
}

function renderSwitchGrid(switches) {
  switchesGrid.textContent = '';

  if (!switches || switches.length === 0) {
    const empty = el('div', 'empty-state');
    append(empty,
      el('div', 'empty-state-icon', '🔌'),
      el('div', 'empty-state-title', 'No switches yet'),
      el('div', 'empty-state-desc', 'Add your first TP-Link smart switch to get started.'),
    );
    const action = el('div', 'empty-state-action');
    const addBtn = el('button', 'btn btn-primary', 'Add Switch');
    addBtn.addEventListener('click', openAddModal);
    action.appendChild(addBtn);
    empty.appendChild(action);
    switchesGrid.appendChild(empty);
    return;
  }

  for (const sw of switches) {
    switchesGrid.appendChild(buildSwitchCard(sw));
  }
}

function buildSwitchCard(sw) {
  const card = el('div', 'switch-card');
  card.dataset.id = sw.id;

  // ── Card body
  const body = el('div', 'switch-card-body');

  const header = el('div', 'switch-card-header');
  const info = el('div');
  append(info,
    el('div', 'switch-name', sw.name),
    el('div', 'switch-ip',   sw.ip),
  );
  if (sw.model) info.appendChild(el('div', 'switch-model', sw.model));
  append(header, info, statusBadgeEl(sw.status, sw.collecting_since));
  body.appendChild(header);

  // Port counts
  const up    = sw.ports_up    ?? 0;
  const down  = sw.ports_down  ?? 0;
  const total = sw.ports_total ?? 0;

  const stats = el('div', 'switch-stats');
  for (const [val, label, cls] of [
    [up,    'Up',    'stat-up'],
    [down,  'Down',  'stat-down'],
    [total, 'Total', 'stat-total'],
  ]) {
    const stat = el('div', 'switch-stat');
    append(stat,
      el('div', `switch-stat-value ${cls}`, String(val)),
      el('div', 'switch-stat-label', label),
    );
    stats.appendChild(stat);
  }
  body.appendChild(stats);
  card.appendChild(body);

  // ── Port LED strip — per-port actual status when available
  const ledStrip = el('div', 'port-led-strip');
  const ledLabel = el('span', 'port-led-label', 'Ports');
  const leds = el('div', 'port-leds');
  const states = sw.port_states || [];
  const portCount = states.length || total || 8;
  for (let i = 0; i < portCount; i++) {
    const st = states[i] || 'down';
    const cls = st === 'up' ? 'led-up' : st === 'disabled' ? 'led-disabled' : 'led-down';
    const led = el('div', `port-led ${cls}`);
    led.title = `Port ${i + 1}: ${st}`;
    leds.appendChild(led);
  }
  append(ledStrip, ledLabel, leds);
  card.appendChild(ledStrip);

  // ── Actions
  const actions = el('div', 'switch-card-actions');

  const editBtn = el('button', 'btn btn-ghost btn-sm', 'Edit');
  editBtn.dataset.action = 'edit';

  const collectBtn = el('button', 'btn btn-ghost btn-sm', 'Collect');
  collectBtn.dataset.action = 'collect';

  const deleteBtn = el('button', 'btn btn-danger btn-sm', 'Delete');
  deleteBtn.dataset.action = 'delete';
  deleteBtn.dataset.name = sw.name;

  append(actions, editBtn, collectBtn, deleteBtn);
  card.appendChild(actions);

  // Navigate on card click (but not on action buttons)
  card.addEventListener('click', e => {
    const btn = e.target.closest('[data-action]');
    if (btn) return;
    location.href = '/switches/' + encodeURIComponent(sw.id);
  });

  editBtn.addEventListener('click', e => {
    e.stopPropagation();
    openEditModal(sw.id);
  });

  collectBtn.addEventListener('click', e => {
    e.stopPropagation();
    collectSwitchCard(sw.id, collectBtn);
  });

  deleteBtn.addEventListener('click', e => {
    e.stopPropagation();
    deleteSwitchById(sw.id, sw.name);
  });

  return card;
}

async function collectSwitchCard(id, btn) {
  btn.disabled = true;
  btn.textContent = 'Collecting…';
  try {
    await apiPost(`/switches/${id}/collect`);
    await loadSwitches();
    startCountdownTick();
    scheduleCollectingPoll();
  } catch (e) {
    toast('Collection failed: ' + e.message, 'danger');
    btn.disabled = false;
    btn.textContent = 'Collect';
  }
  // Button re-enables automatically when loadSwitches re-renders the card.
}

async function deleteSwitchById(id, name) {
  if (!confirm(`Delete switch "${name}"? This cannot be undone.`)) return;
  try {
    await apiDel(`/switches/${id}`);
    toast('Switch deleted');
    await loadSwitches();
  } catch (e) {
    toast('Delete failed: ' + e.message, 'danger');
  }
}

// ── Add / Edit modal ──────────────────────────────────────────────────────

let editingSwitchId = null;

function openAddModal() {
  editingSwitchId = null;
  setText('modal-title', 'Add Switch');
  document.getElementById('switch-form').reset();
  document.getElementById('modal-pwd-hint').classList.remove('hidden');
  document.getElementById('modal-backdrop').classList.remove('hidden');
}

async function openEditModal(id) {
  editingSwitchId = id;
  setText('modal-title', 'Edit Switch');
  document.getElementById('modal-pwd-hint').classList.remove('hidden');
  document.getElementById('modal-backdrop').classList.remove('hidden');
  try {
    const sw = await apiGet(`/switches/${id}`);
    document.getElementById('f-name').value       = sw.name || '';
    document.getElementById('f-ip').value          = sw.ip || '';
    document.getElementById('f-username').value    = sw.username || '';
    document.getElementById('f-password').value    = '';
    document.getElementById('f-insecure').checked  = sw.insecure_tls || false;
    document.getElementById('f-poll-stats').value  = sw.poll_stats_secs || 60;
    document.getElementById('f-poll-config').value = sw.poll_config_secs || 300;
  } catch (e) {
    toast('Load failed: ' + e.message, 'danger');
    closeModal();
  }
}

function closeModal() {
  document.getElementById('modal-backdrop').classList.add('hidden');
  editingSwitchId = null;
}

document.getElementById('modal-backdrop')?.addEventListener('click', e => {
  if (e.target === e.currentTarget) closeModal();
});

document.getElementById('btn-close-modal')?.addEventListener('click', closeModal);
document.getElementById('btn-cancel-modal')?.addEventListener('click', closeModal);

document.getElementById('switch-form')?.addEventListener('submit', async e => {
  e.preventDefault();
  const btn = e.target.querySelector('[type=submit]');
  btn.disabled = true;

  const body = {
    name:             document.getElementById('f-name').value.trim(),
    ip:               document.getElementById('f-ip').value.trim(),
    username:         document.getElementById('f-username').value.trim(),
    password:         document.getElementById('f-password').value,
    insecure_tls:     document.getElementById('f-insecure').checked,
    poll_stats_secs:  parseInt(document.getElementById('f-poll-stats').value) || 60,
    poll_config_secs: parseInt(document.getElementById('f-poll-config').value) || 300,
  };
  if (editingSwitchId) body.enabled = true;

  try {
    if (editingSwitchId) {
      await apiPut(`/switches/${editingSwitchId}`, body);
      toast('Switch updated');
      closeModal();
      await loadSwitches();
    } else {
      await apiPost('/switches', body);
      toast('Switch added — collecting data…');
      closeModal();
      await loadSwitches();
      startCountdownTick();
      scheduleCollectingPoll();
    }
  } catch (err) {
    toast(err.message, 'danger');
  } finally {
    btn.disabled = false;
  }
});

// "Add Switch" button in page header (dashboard)
document.getElementById('btn-add-switch')?.addEventListener('click', openAddModal);

// ══════════════════════════════════════════════════════════════════════════
// SWITCH DETAIL
// ══════════════════════════════════════════════════════════════════════════

const switchDetailEl = document.getElementById('switch-detail');
if (switchDetailEl) {
  const switchId = switchDetailEl.dataset.switchId;
  initSwitchDetail(switchId);
}

async function initSwitchDetail(id) {
  try {
    const sw = await apiGet(`/switches/${id}`);
    setText('sw-name', sw.name);
    const statusEl = document.getElementById('sw-status');
    if (statusEl) {
      statusEl.textContent = '';
      statusEl.appendChild(statusBadgeEl(sw.status, sw.collecting_since));
    }
    await loadSnapshot(id);
    setupTabs();
  } catch (e) {
    setText('sw-name', 'Error loading switch');
    toast('Failed to load switch: ' + e.message, 'danger');
  }
}

async function loadSnapshot(id) {
  try {
    const snap = await apiGet(`/switches/${id}/snapshot`);
    if (!snap) { renderNoSnapshot(); return; }
    renderSystemTab(snap);
    renderPortsTab(snap, id);
    renderVLANsTab(snap);
    await renderStatsTab(id);
  } catch {
    renderNoSnapshot();
  }
}

function renderNoSnapshot() {
  const t = document.getElementById('tab-system');
  if (!t) return;
  t.textContent = '';
  const empty = el('div', 'empty-state');
  append(empty,
    el('div', 'empty-state-icon', '📡'),
    el('div', 'empty-state-title', 'No data collected yet'),
    el('div', 'empty-state-desc', 'Click "Collect Now" to fetch the switch state.'),
  );
  t.appendChild(empty);
}

function renderSystemTab(snap) {
  const container = document.getElementById('tab-system');
  if (!container) return;
  container.textContent = '';

  const sw = snap.switch || {};
  const fields = [
    ['Model',     sw.model],
    ['Hardware',  sw.hardware],
    ['Firmware',  sw.firmware],
    ['Serial',    sw.serial],
    ['MAC',       sw.mac],
    ['Uptime',    formatDuration(sw.uptime)],
    ['Location',  sw.location],
    ['Collected', snap.collected_at ? new Date(snap.collected_at).toLocaleString() : null],
  ].filter(([, v]) => v);

  if (!fields.length) {
    container.appendChild(el('p', 'text-muted', 'No system data available.'));
    return;
  }

  const grid = el('div', 'info-grid');
  for (const [label, value] of fields) {
    const item = el('div', 'info-item');
    append(item, el('div', 'info-label', label), el('div', 'info-value', value));
    grid.appendChild(item);
  }
  container.appendChild(grid);
}

function renderPortsTab(snap, switchId) {
  const container = document.getElementById('tab-ports');
  if (!container) return;
  container.textContent = '';

  const ports = snap.ports || [];
  if (!ports.length) {
    container.appendChild(el('p', 'text-muted', 'No port data.'));
    return;
  }

  const wrapper = el('div', 'table-wrapper');
  const table   = el('table');
  const thead   = el('thead');
  const hr      = el('tr');
  for (const h of ['Port', 'Status', 'Speed', 'Duplex', 'Description', 'Enabled']) {
    hr.appendChild(el('th', null, h));
  }
  thead.appendChild(hr);
  table.appendChild(thead);

  const tbody = el('tbody');
  for (const p of ports) {
    const row = el('tr');

    const numCell = el('td', 'port-num', String(p.number));
    const statusCell = el('td');
    statusCell.appendChild(portStatusEl(p.status));
    const speedCell = el('td', null, p.speed || '—');
    const duplexCell = el('td', null, p.duplex || '—');
    const descCell = el('td', null, p.description || '—');

    const toggleCell = el('td');
    const label = el('label', 'toggle');
    label.title = p.enabled ? 'Disable port' : 'Enable port';
    const checkbox = el('input');
    checkbox.type = 'checkbox';
    checkbox.checked = p.enabled;
    checkbox.addEventListener('change', () => togglePort(switchId, p.number, checkbox));
    const track = el('span', 'toggle-track');
    append(label, checkbox, track);
    toggleCell.appendChild(label);

    append(row, numCell, statusCell, speedCell, duplexCell, descCell, toggleCell);
    tbody.appendChild(row);
  }
  table.appendChild(tbody);
  wrapper.appendChild(table);
  container.appendChild(wrapper);
}

async function togglePort(switchId, portNum, checkbox) {
  const enabled = checkbox.checked;
  checkbox.disabled = true;
  try {
    const res = await apiPatch(`/switches/${switchId}/ports/${portNum}`, { enabled });
    toast(`Port ${portNum} ${enabled ? 'enabled' : 'disabled'}`);
    if (res && res.ports) {
      renderPortsTab({ ports: res.ports }, switchId);
    }
  } catch (e) {
    checkbox.checked = !enabled;
    toast('Failed: ' + e.message, 'danger');
  } finally {
    checkbox.disabled = false;
  }
}

async function renderStatsTab(id) {
  const container = document.getElementById('tab-stats');
  if (!container) return;
  container.textContent = '';
  try {
    const stats = await apiGet(`/switches/${id}/stats`);
    if (!stats || !stats.length) {
      container.appendChild(el('p', 'text-muted', 'No statistics data.'));
      return;
    }
    const wrapper = el('div', 'table-wrapper');
    const table   = el('table');
    const thead   = el('thead');
    const hr      = el('tr');
    for (const h of ['Port', 'RX Bytes', 'TX Bytes', 'RX Pkts', 'TX Pkts', 'RX Err', 'TX Err']) {
      hr.appendChild(el('th', null, h));
    }
    thead.appendChild(hr);
    table.appendChild(thead);

    const tbody = el('tbody');
    for (const s of stats) {
      const row = el('tr');
      const cells = [
        el('td', 'port-num', String(s.port_number)),
        el('td', 'text-mono text-sm', fmtBytes(s.rx_bytes)),
        el('td', 'text-mono text-sm', fmtBytes(s.tx_bytes)),
        el('td', 'text-mono text-sm', fmtNum(s.rx_packets)),
        el('td', 'text-mono text-sm', fmtNum(s.tx_packets)),
        el('td', 'text-mono text-sm text-muted', fmtNum(s.rx_errors)),
        el('td', 'text-mono text-sm text-muted', fmtNum(s.tx_errors)),
      ];
      for (const c of cells) row.appendChild(c);
      tbody.appendChild(row);
    }
    table.appendChild(tbody);
    wrapper.appendChild(table);
    container.appendChild(wrapper);
  } catch {
    container.appendChild(el('p', 'text-muted', 'No statistics available.'));
  }
}

function renderVLANsTab(snap) {
  const container = document.getElementById('tab-vlans');
  if (!container) return;
  container.textContent = '';

  const vlans = snap.vlans || [];
  if (!vlans.length) {
    container.appendChild(el('p', 'text-muted', 'No VLAN data.'));
    return;
  }

  const wrapper = el('div', 'table-wrapper');
  const table   = el('table');
  const thead   = el('thead');
  const hr      = el('tr');
  for (const h of ['VLAN ID', 'Name', 'Members']) hr.appendChild(el('th', null, h));
  thead.appendChild(hr);
  table.appendChild(thead);

  const tbody = el('tbody');
  for (const v of vlans) {
    const row = el('tr');
    row.appendChild(el('td', 'port-num', String(v.id)));
    row.appendChild(el('td', null, v.name || '—'));

    const membersCell = el('td');
    const membersWrap = el('div', 'vlan-port-cell');
    const members = v.port_members || {};
    for (const [port, mode] of Object.entries(members)) {
      const tag = el('span', `port-tag port-tag-${mode === 'tagged' ? 'tagged' : 'untagged'}`, port);
      tag.title = mode;
      membersWrap.appendChild(tag);
    }
    if (!Object.keys(members).length) membersWrap.textContent = '—';
    membersCell.appendChild(membersWrap);
    row.appendChild(membersCell);
    tbody.appendChild(row);
  }
  table.appendChild(tbody);
  wrapper.appendChild(table);
  container.appendChild(wrapper);
}

function renderPoETab(snap) {
  const container = document.getElementById('tab-poe');
  if (!container) return;
  container.textContent = '';

  const poe = snap.poe;
  if (!poe) {
    container.appendChild(el('p', 'text-muted', 'PoE not available on this switch.'));
    return;
  }

  // Budget stats
  const statsRow = el('div', 'stats-row');
  statsRow.style.marginBottom = '16px';
  for (const [label, value] of [
    ['Total Budget', poe.total_budget_w != null ? poe.total_budget_w.toFixed(1) + ' W' : '—'],
    ['Consumed',     poe.consumed_watts  != null ? poe.consumed_watts.toFixed(1)  + ' W' : '—'],
    ['Remaining',    poe.remaining_watts != null ? poe.remaining_watts.toFixed(1) + ' W' : '—'],
  ]) {
    const card = el('div', 'stat-card');
    append(card, el('div', 'stat-card-label', label), el('div', 'stat-card-value', value));
    statsRow.appendChild(card);
  }
  container.appendChild(statsRow);

  // Per-port table
  const wrapper = el('div', 'table-wrapper');
  const table   = el('table');
  const thead   = el('thead');
  const hr      = el('tr');
  for (const h of ['Port', 'Status', 'Priority', 'Consumption', 'Limit', 'Class']) {
    hr.appendChild(el('th', null, h));
  }
  thead.appendChild(hr);
  table.appendChild(thead);

  const tbody = el('tbody');
  for (const p of poe.ports || []) {
    const row = el('tr');
    row.appendChild(el('td', 'port-num', String(p.port_number)));

    const statusCell = el('td');
    statusCell.appendChild(statusBadgeEl(p.enabled ? 'online' : 'unknown'));
    row.appendChild(statusCell);

    row.appendChild(el('td', null, p.priority || '—'));
    row.appendChild(el('td', 'text-mono text-sm', p.power_watts  != null ? p.power_watts.toFixed(1)  + ' W' : '—'));
    row.appendChild(el('td', 'text-mono text-sm', p.power_limit_w != null ? p.power_limit_w.toFixed(1) + ' W' : '—'));
    row.appendChild(el('td', null, p.class != null ? String(p.class) : '—'));
    tbody.appendChild(row);
  }
  table.appendChild(tbody);
  wrapper.appendChild(table);
  container.appendChild(wrapper);
}

function setupTabs() {
  const buttons = document.querySelectorAll('.tab-btn');
  const panels  = document.querySelectorAll('.tab-panel');

  buttons.forEach(btn => {
    btn.addEventListener('click', () => {
      buttons.forEach(b => b.classList.remove('active'));
      panels.forEach(p => p.classList.remove('active'));
      btn.classList.add('active');
      const target = document.getElementById('tab-' + btn.dataset.tab);
      if (target) target.classList.add('active');
    });
  });

  if (buttons.length) buttons[0].click();
}

// ── Switch detail action buttons ──────────────────────────────────────────

document.getElementById('btn-collect')?.addEventListener('click', async function () {
  const id = document.getElementById('switch-detail')?.dataset.switchId;
  if (!id) return;
  this.disabled = true;
  this.textContent = 'Collecting…';
  try {
    const res = await apiPost(`/switches/${id}/collect`);
    // Update the status badge immediately with the countdown.
    const statusEl = document.getElementById('sw-status');
    if (statusEl) {
      statusEl.textContent = '';
      statusEl.appendChild(statusBadgeEl('collecting', res?.collecting_since));
    }
    startCountdownTick();
    pollDetailCollection(id, this);
  } catch (e) {
    toast('Collection failed: ' + e.message, 'danger');
    this.disabled = false;
    this.textContent = 'Collect Now';
  }
});

async function pollDetailCollection(id, btn) {
  const sw = await apiGet(`/switches/${id}`).catch(() => null);
  if (!sw) return;
  const statusEl = document.getElementById('sw-status');
  if (statusEl) {
    statusEl.textContent = '';
    statusEl.appendChild(statusBadgeEl(sw.status, sw.collecting_since));
  }
  if (sw.status === 'collecting') {
    setTimeout(() => pollDetailCollection(id, btn), 2000);
  } else {
    stopCountdownTick();
    if (btn) { btn.disabled = false; btn.textContent = 'Collect Now'; }
    await loadSnapshot(id);
    toast('Collection complete — data refreshed');
  }
}

document.getElementById('btn-reboot')?.addEventListener('click', async function () {
  const id = document.getElementById('switch-detail')?.dataset.switchId;
  if (!id) return;
  if (!confirm('Reboot this switch? It will be unreachable for a short time.')) return;
  try {
    await apiPost(`/switches/${id}/reboot`);
    toast('Reboot command sent');
  } catch (e) {
    toast('Reboot failed: ' + e.message, 'danger');
  }
});

// ══════════════════════════════════════════════════════════════════════════
// SETTINGS
// ══════════════════════════════════════════════════════════════════════════

const settingsFormEl = document.getElementById('credentials-form');
if (settingsFormEl) initSettings();

async function initSettings() {
  try {
    const s = await apiGet('/settings');
    const authEl = document.getElementById('s-auth-enabled');
    if (authEl) authEl.checked = s.auth_enabled;
    await loadTokenList();
    const sess = await apiGet('/auth/session').catch(() => null);
    if (sess?.authenticated) {
      document.getElementById('logout-row')?.classList.remove('hidden');
    }
  } catch (e) {
    toast('Failed to load settings: ' + e.message, 'danger');
  }
}

// ── API Token list ─────────────────────────────────────────────────────────

async function loadTokenList() {
  const wrap = document.getElementById('token-list-wrap');
  if (!wrap) return;
  try {
    const tokens = await apiGet('/settings/tokens');
    renderTokenList(wrap, tokens || []);
  } catch (e) {
    wrap.textContent = '';
    const err = el('p', 'text-muted', 'Failed to load tokens: ' + e.message);
    err.style.padding = '12px';
    wrap.appendChild(err);
  }
}

function renderTokenList(wrap, tokens) {
  wrap.textContent = '';
  if (!tokens.length) {
    const empty = el('p', 'text-muted', 'No tokens yet. Add one to allow external API access.');
    empty.style.padding = '16px';
    wrap.appendChild(empty);
    return;
  }
  const table = el('table');
  const thead = el('thead');
  const hr = el('tr');
  for (const h of ['Name', 'Expiry', 'Created', '']) hr.appendChild(el('th', null, h));
  thead.appendChild(hr); table.appendChild(thead);
  const tbody = el('tbody');
  for (const t of tokens) {
    const row = el('tr');
    row.appendChild(el('td', 'font-bold', t.name));
    row.appendChild(el('td', 'text-mono text-sm',
      t.expiry ? new Date(t.expiry * 1000).toLocaleDateString() : 'Never'));
    row.appendChild(el('td', 'text-sm text-muted',
      new Date(t.created_at).toLocaleDateString()));
    const act = el('td');
    const del = el('button', 'btn btn-danger btn-sm', 'Revoke');
    del.addEventListener('click', () => revokeToken(t.id, t.name));
    act.appendChild(del);
    row.appendChild(act);
    tbody.appendChild(row);
  }
  table.appendChild(tbody);
  const wrapper = el('div', 'table-wrapper');
  wrapper.appendChild(table);
  wrap.appendChild(wrapper);
}

async function revokeToken(id, name) {
  if (!confirm(`Revoke token "${name}"? Any API clients using it will stop working.`)) return;
  try {
    await apiDel(`/settings/tokens/${id}`);
    toast('Token revoked');
    await loadTokenList();
  } catch (e) {
    toast(e.message, 'danger');
  }
}

// ── Add token modal ────────────────────────────────────────────────────────

document.getElementById('btn-add-token')?.addEventListener('click', openAddTokenModal);
document.getElementById('btn-close-token-modal')?.addEventListener('click', closeTokenModal);
document.getElementById('btn-cancel-token-modal')?.addEventListener('click', closeTokenModal);
document.getElementById('token-modal-backdrop')?.addEventListener('click', e => {
  if (e.target === e.currentTarget) closeTokenModal();
});

document.getElementById('t-expiry')?.addEventListener('change', function () {
  document.getElementById('t-expiry-custom-wrap')?.classList.toggle('hidden', this.value !== 'custom');
});

function openAddTokenModal() {
  document.getElementById('token-create-form-wrap')?.classList.remove('hidden');
  document.getElementById('token-reveal-wrap')?.classList.add('hidden');
  document.getElementById('t-name').value = '';
  document.getElementById('t-expiry').value = '0';
  document.getElementById('t-expiry-custom-wrap')?.classList.add('hidden');
  const footer = document.getElementById('token-modal-footer');
  if (footer) {
    footer.textContent = '';
    const cancelBtn = el('button', 'btn btn-secondary', 'Cancel');
    cancelBtn.type = 'button';
    const createBtn = el('button', 'btn btn-primary', 'Create Token');
    createBtn.type = 'button';
    cancelBtn.addEventListener('click', closeTokenModal);
    createBtn.addEventListener('click', doCreateToken);
    append(footer, cancelBtn, createBtn);
  }
  document.getElementById('token-modal-backdrop')?.classList.remove('hidden');
}

function closeTokenModal() {
  document.getElementById('token-modal-backdrop')?.classList.add('hidden');
}

async function doCreateToken() {
  const name = document.getElementById('t-name').value.trim();
  if (!name) { toast('Token name is required', 'danger'); return; }
  const sel = document.getElementById('t-expiry').value;
  let expiry = 0;
  if (sel === 'custom') {
    const val = document.getElementById('t-expiry-custom').value;
    if (!val) { toast('Pick a custom date', 'danger'); return; }
    expiry = Math.floor(new Date(val).getTime() / 1000);
  } else {
    const secs = parseInt(sel) || 0;
    expiry = secs > 0 ? Math.floor(Date.now() / 1000) + secs : 0;
  }
  const btn = document.getElementById('btn-create-token');
  if (btn) btn.disabled = true;
  try {
    const res = await apiPost('/settings/tokens', { name, expiry });
    // Show reveal phase
    document.getElementById('token-create-form-wrap')?.classList.add('hidden');
    document.getElementById('token-reveal-wrap')?.classList.remove('hidden');
    const field = document.getElementById('t-reveal');
    if (field) field.value = res.token;
    await navigator.clipboard.writeText(res.token).catch(() => {});
    const footer = document.getElementById('token-modal-footer');
    if (footer) {
      footer.textContent = '';
      const doneBtn = el('button', 'btn btn-primary', "Done — I've saved it");
      doneBtn.type = 'button';
      doneBtn.addEventListener('click', async () => {
        closeTokenModal();
        await loadTokenList();
      });
      footer.appendChild(doneBtn);
    }
    document.getElementById('btn-copy-token')?.addEventListener('click', async () => {
      const f = document.getElementById('t-reveal');
      await navigator.clipboard.writeText(f.value).catch(() => {});
      toast('Copied!');
    });
  } catch (e) {
    toast(e.message, 'danger');
    if (btn) btn.disabled = false;
  }
}

document.getElementById('s-auth-enabled')?.addEventListener('change', async function () {
  try {
    await apiPut('/settings', { auth_enabled: this.checked });
    toast(this.checked ? 'Authentication enabled' : 'Authentication disabled');
  } catch (e) {
    toast(e.message, 'danger');
    this.checked = !this.checked;
  }
});

document.getElementById('credentials-form')?.addEventListener('submit', async e => {
  e.preventDefault();
  const btn = e.target.querySelector('[type=submit]');
  btn.disabled = true;
  const username = document.getElementById('s-username').value.trim();
  const password = document.getElementById('s-password').value;
  const password2 = document.getElementById('s-password2').value;
  if (!username || !password) { toast('Username and password are required', 'danger'); btn.disabled = false; return; }
  if (password !== password2) { toast('Passwords do not match', 'danger'); btn.disabled = false; return; }
  if (password.length < 8) { toast('Password must be at least 8 characters', 'danger'); btn.disabled = false; return; }
  try {
    await apiPut('/settings', { username, password });
    toast('Credentials saved');
    document.getElementById('s-password').value = '';
    document.getElementById('s-password2').value = '';
  } catch (err) {
    toast(err.message, 'danger');
  } finally {
    btn.disabled = false;
  }
});

document.getElementById('btn-logout')?.addEventListener('click', async () => {
  try {
    await apiPost('/auth/logout', {});
    window.location.href = '/login';
  } catch (e) {
    toast(e.message, 'danger');
  }
});

// ══════════════════════════════════════════════════════════════════════════
// NOTIFICATIONS
// ══════════════════════════════════════════════════════════════════════════

const notifList = document.getElementById('notif-list');
if (notifList) initNotifications();

async function initNotifications() {
  await loadNotifications();
}

async function loadNotifications() {
  try {
    const channels = await apiGet('/notifications');
    renderNotifList(channels || []);
  } catch (e) {
    const errEl = el('p', 'alert alert-danger');
    errEl.style.margin = '12px';
    errEl.textContent = 'Failed to load: ' + e.message;
    notifList.textContent = '';
    notifList.appendChild(errEl);
  }
}

function renderNotifList(channels) {
  notifList.textContent = '';
  if (!channels.length) {
    const empty = el('p', 'text-muted', 'No notification channels configured.');
    empty.style.padding = '16px';
    notifList.appendChild(empty);
    return;
  }
  const wrapper = el('div', 'table-wrapper');
  const table = el('table');
  const thead = el('thead');
  const hr = el('tr');
  for (const h of ['Name', 'Provider', 'Offline', 'Online', 'Enabled', '']) {
    hr.appendChild(el('th', null, h));
  }
  thead.appendChild(hr); table.appendChild(thead);
  const tbody = el('tbody');
  for (const ch of channels) {
    const row = el('tr');
    row.appendChild(el('td', null, ch.name));
    row.appendChild(el('td', null, ch.provider));
    row.appendChild(el('td', null, ch.notify_offline ? '✓' : '—'));
    row.appendChild(el('td', null, ch.notify_online ? '✓' : '—'));
    row.appendChild(el('td', null, ch.enabled ? '✓' : '—'));
    const actions = el('td');
    const editBtn = el('button', 'btn btn-ghost btn-sm', 'Edit');
    editBtn.addEventListener('click', () => openEditNotifModal(ch));
    const delBtn = el('button', 'btn btn-danger btn-sm', 'Delete');
    delBtn.style.marginLeft = '6px';
    delBtn.addEventListener('click', () => deleteNotif(ch.id, ch.name));
    append(actions, editBtn, delBtn);
    row.appendChild(actions);
    tbody.appendChild(row);
  }
  table.appendChild(tbody);
  wrapper.appendChild(table);
  notifList.appendChild(wrapper);
}

// ── Provider field toggling ───────────────────────────────────────────────

document.getElementById('nf-provider')?.addEventListener('change', function () {
  switchNotifFields(this.value);
});

function switchNotifFields(provider) {
  ['shoutrrr', 'greenapi', 'whatsapp_web'].forEach(p => {
    const divEl = document.getElementById('nf-fields-' + p);
    if (divEl) divEl.classList.toggle('hidden', p !== provider);
  });
}

// ── Modal helpers ──────────────────────────────────────────────────────────

let editingNotifId = null;

function openAddNotifModal() {
  editingNotifId = null;
  setText('notif-modal-title', 'Add Channel');
  document.getElementById('notif-form').reset();
  switchNotifFields('shoutrrr');
  document.getElementById('nf-test-result').classList.add('hidden');
  document.getElementById('notif-modal-backdrop').classList.remove('hidden');
}

function openEditNotifModal(ch) {
  editingNotifId = ch.id;
  setText('notif-modal-title', 'Edit Channel');
  document.getElementById('nf-name').value = ch.name || '';
  document.getElementById('nf-provider').value = ch.provider || 'shoutrrr';
  switchNotifFields(ch.provider);
  try {
    const cfg = JSON.parse(ch.config || '{}');
    if (ch.provider === 'shoutrrr') {
      document.getElementById('nf-url').value = cfg.url || '';
    } else if (ch.provider === 'greenapi') {
      document.getElementById('nf-instance-id').value = cfg.instance_id || '';
      document.getElementById('nf-ga-token').value = cfg.token || '';
      document.getElementById('nf-recipient').value = cfg.recipient || '';
      document.getElementById('nf-api-url').value = cfg.api_url || '';
    } else if (ch.provider === 'whatsapp_web') {
      document.getElementById('nf-base-url').value = cfg.base_url || '';
      document.getElementById('nf-wa-recipient').value = cfg.recipient || '';
      document.getElementById('nf-wa-username').value = cfg.username || '';
      document.getElementById('nf-wa-password').value = cfg.password || '';
    }
  } catch {}
  document.getElementById('nf-notify-offline').checked = ch.notify_offline;
  document.getElementById('nf-notify-online').checked = ch.notify_online;
  document.getElementById('nf-enabled').checked = ch.enabled;
  document.getElementById('nf-test-result').classList.add('hidden');
  document.getElementById('notif-modal-backdrop').classList.remove('hidden');
}

function closeNotifModal() {
  document.getElementById('notif-modal-backdrop').classList.add('hidden');
  editingNotifId = null;
}

function buildNotifConfig(provider) {
  if (provider === 'shoutrrr') {
    return JSON.stringify({ url: document.getElementById('nf-url').value.trim() });
  }
  if (provider === 'greenapi') {
    return JSON.stringify({
      instance_id: document.getElementById('nf-instance-id').value.trim(),
      token:       document.getElementById('nf-ga-token').value.trim(),
      recipient:   document.getElementById('nf-recipient').value.trim(),
      api_url:     document.getElementById('nf-api-url').value.trim(),
    });
  }
  return JSON.stringify({
    base_url:  document.getElementById('nf-base-url').value.trim(),
    recipient: document.getElementById('nf-wa-recipient').value.trim(),
    username:  document.getElementById('nf-wa-username').value.trim(),
    password:  document.getElementById('nf-wa-password').value.trim(),
  });
}

// ── Event handlers ─────────────────────────────────────────────────────────

document.getElementById('btn-add-notif')?.addEventListener('click', openAddNotifModal);
document.getElementById('btn-close-notif-modal')?.addEventListener('click', closeNotifModal);
document.getElementById('btn-cancel-notif-modal')?.addEventListener('click', closeNotifModal);
document.getElementById('notif-modal-backdrop')?.addEventListener('click', e => {
  if (e.target === e.currentTarget) closeNotifModal();
});

document.getElementById('notif-form')?.addEventListener('submit', async e => {
  e.preventDefault();
  const btn = e.target.querySelector('[type=submit]');
  btn.disabled = true;
  const provider = document.getElementById('nf-provider').value;
  const body = {
    name:           document.getElementById('nf-name').value.trim(),
    provider,
    config:         buildNotifConfig(provider),
    notify_offline: document.getElementById('nf-notify-offline').checked,
    notify_online:  document.getElementById('nf-notify-online').checked,
    enabled:        document.getElementById('nf-enabled').checked,
  };
  try {
    if (editingNotifId) {
      await apiPut('/notifications/' + editingNotifId, body);
      toast('Channel updated');
    } else {
      await apiPost('/notifications', body);
      toast('Channel added');
    }
    closeNotifModal();
    await loadNotifications();
  } catch (err) {
    toast(err.message, 'danger');
  } finally {
    btn.disabled = false;
  }
});

document.getElementById('btn-test-notif')?.addEventListener('click', async function () {
  const provider = document.getElementById('nf-provider').value;
  const resultEl = document.getElementById('nf-test-result');
  this.disabled = true;
  this.textContent = 'Sending…';
  resultEl.className = 'hidden';
  try {
    await apiPost('/notifications/test', {
      provider,
      config: buildNotifConfig(provider),
    });
    resultEl.className = 'alert alert-success';
    resultEl.textContent = '✓ Test message sent successfully';
  } catch (err) {
    resultEl.className = 'alert alert-danger';
    resultEl.textContent = 'Error: ' + err.message;
  } finally {
    this.disabled = false;
    this.textContent = 'Send Test';
  }
});

async function deleteNotif(id, name) {
  if (!confirm('Delete channel "' + name + '"?')) return;
  try {
    await apiDel('/notifications/' + id);
    toast('Channel deleted');
    await loadNotifications();
  } catch (e) {
    toast('Delete failed: ' + e.message, 'danger');
  }
}
