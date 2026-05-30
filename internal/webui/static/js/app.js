'use strict';

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
  document.querySelectorAll('.nav-link').forEach(a => {
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
  card.style.cursor = 'pointer';

  // Header
  const header = el('div', 'switch-card-header');
  const info = el('div');
  const name = el('div', 'switch-name', sw.name);
  const ip   = el('div', 'switch-ip',   sw.ip);
  append(info, name, ip);
  if (sw.model) append(info, el('div', 'text-sm text-muted mt-1', sw.model));
  append(header, info, statusBadgeEl(sw.status, sw.collecting_since));
  card.appendChild(header);

  // Port stats
  const meta = el('div', 'switch-meta');
  const up    = sw.ports_up    ?? 0;
  const down  = sw.ports_down  ?? 0;
  const total = sw.ports_total ?? 0;

  for (const [val, label, cls] of [[up, 'Up', 'stat-up'], [down, 'Down', 'stat-down'], [total, 'Total', 'stat-total']]) {
    const stat = el('div', 'switch-stat');
    const v = el('div', `switch-stat-value ${cls}`, String(val));
    const l = el('div', 'switch-stat-label', label);
    append(stat, v, l);
    meta.appendChild(stat);
  }
  card.appendChild(meta);

  // Actions
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

const settingsFormEl = document.getElementById('settings-form');
if (settingsFormEl) initSettings();

async function initSettings() {
  try {
    const s = await apiGet('/settings');
    document.getElementById('s-auth-enabled').checked = s.auth_enabled === 'true';
    document.getElementById('s-auth-token').value = s.auth_token || '';
  } catch (e) {
    toast('Failed to load settings: ' + e.message, 'danger');
  }
}

settingsFormEl?.addEventListener('submit', async e => {
  e.preventDefault();
  const btn = e.target.querySelector('[type=submit]');
  btn.disabled = true;
  try {
    await apiPut('/settings', {
      auth_enabled: document.getElementById('s-auth-enabled').checked ? 'true' : 'false',
      auth_token:   document.getElementById('s-auth-token').value,
    });
    toast('Settings saved');
  } catch (err) {
    toast(err.message, 'danger');
  } finally {
    btn.disabled = false;
  }
});

document.getElementById('btn-gen-token')?.addEventListener('click', () => {
  const arr = new Uint8Array(24);
  crypto.getRandomValues(arr);
  document.getElementById('s-auth-token').value =
    Array.from(arr).map(b => b.toString(16).padStart(2, '0')).join('');
});
