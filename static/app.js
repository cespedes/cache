const API = '';  // same origin

// ── State ──────────────────────────────────────────────────────────────────
let currentTab = 'locations';
let locations = [];
let items = [];
let selected = null;
let formMode = null;   // { action: 'create'|'edit', entity, data?, defaultLocationId? }
let searchTimer = null;
let expandedIds = new Set(); // ids of expanded location nodes

// ── Helpers ────────────────────────────────────────────────────────────────
async function api(method, path, body) {
  const opts = { method, headers: { 'Content-Type': 'application/json' } };
  if (body) opts.body = JSON.stringify(body);
  const res = await fetch(API + path, opts);
  if (!res.ok) throw new Error(await res.text());
  if (res.status === 204) return null;
  return res.json();
}

function toast(msg, error = false) {
  const el = document.getElementById('toast');
  el.textContent = msg;
  el.className = 'toast show' + (error ? ' error' : '');
  clearTimeout(el._t);
  el._t = setTimeout(() => el.className = 'toast', 2500);
}

function fmtDate(iso) {
  return new Date(iso).toLocaleString('en-GB', {
    day: '2-digit', month: 'short', year: 'numeric',
    hour: '2-digit', minute: '2-digit'
  });
}

// ── Data loading ───────────────────────────────────────────────────────────
async function loadLocations(q = '') {
  locations = await api('GET', '/locations' + (q ? `?q=${encodeURIComponent(q)}` : ''));
  renderList();
}

async function loadItems(q = '') {
  items = await api('GET', '/items' + (q ? `?q=${encodeURIComponent(q)}` : ''));
  renderList();
}

function load(q = '') {
  if (currentTab === 'locations') loadLocations(q);
  else loadItems(q);
}

// ── Location tree ──────────────────────────────────────────────────────────
function buildTree(locs) {
  const map = {};
  locs.forEach(l => map[l.id] = { ...l, children: [] });
  const roots = [];
  locs.forEach(l => {
    if (l.parent_id && map[l.parent_id]) map[l.parent_id].children.push(map[l.id]);
    else roots.push(map[l.id]);
  });
  return { map, roots };
}

// Flatten only visible nodes (roots + children of expanded nodes)
function flattenVisible(roots, map) {
  const flat = [];
  function walk(node, depth) {
    const hasChildren = node.children.length > 0;
    flat.push({ ...node, depth, hasChildren });
    if (hasChildren && expandedIds.has(node.id)) {
      node.children.forEach(c => walk(map[c.id], depth + 1));
    }
  }
  roots.forEach(r => walk(r, 0));
  return flat;
}

// ── Render list ────────────────────────────────────────────────────────────
function renderList() {
  const el = document.getElementById('list');
  const q = document.getElementById('search').value;

  if (currentTab === 'locations') {
    if (!locations.length) { el.innerHTML = '<div class="list-empty">no locations</div>'; return; }

    let data;
    if (q) {
      // Searching: show flat results with full path as subtitle
      data = locations.map(l => ({ ...l, depth: 0, hasChildren: false, searchPath: locationPath(l.id) }));
    } else {
      const { map, roots } = buildTree(locations);
      data = flattenVisible(roots, map);
    }

    if (!data.length) { el.innerHTML = '<div class="list-empty">no locations</div>'; return; }

    el.innerHTML = data.map(loc => {
      const isExpanded = expandedIds.has(loc.id);
      const toggle = loc.hasChildren
        ? `<span class="tree-toggle${isExpanded ? ' open' : ''}" data-toggle-id="${loc.id}">▶</span>`
        : `<span class="tree-toggle-spacer"></span>`;
      const subtitle = q && loc.searchPath
        ? `<span class="list-item-path">${esc(loc.searchPath)}</span>`
        : '';
      return `<div class="list-item${selected?.id === loc.id ? ' active' : ''}"
                   data-id="${loc.id}" data-depth="${loc.depth || 0}">
        ${toggle}
        <span class="list-item-name">${esc(loc.name)}${subtitle}</span>
      </div>`;
    }).join('');

    // Toggle expand/collapse
    el.querySelectorAll('.tree-toggle').forEach(btn => {
      btn.addEventListener('click', e => {
        e.stopPropagation();
        const id = parseInt(btn.dataset.toggleId);
        if (expandedIds.has(id)) {
          // Collapse this node and all its descendants
          const toCollapse = new Set([id]);
          let changed = true;
          while (changed) {
            changed = false;
            locations.forEach(l => {
              if (!toCollapse.has(l.id) && toCollapse.has(l.parent_id)) {
                toCollapse.add(l.id);
                changed = true;
              }
            });
          }
          toCollapse.forEach(cid => expandedIds.delete(cid));
        } else {
          expandedIds.add(id);
        }
        renderList();
      });
    });

  } else {
    if (!items.length) { el.innerHTML = '<div class="list-empty">no items</div>'; return; }
    el.innerHTML = items.map(item => {
      const loc = locations.find(l => l.id === item.location_id);
      return `<div class="list-item${selected?.id === item.id ? ' active' : ''}" data-id="${item.id}">
        <span class="list-item-name">${esc(item.name)}</span>
        <span class="list-item-meta">${loc ? esc(loc.name) : ''}</span>
      </div>`;
    }).join('');
  }

  el.querySelectorAll('.list-item').forEach(row => {
    row.addEventListener('click', () => selectItem(parseInt(row.dataset.id)));
  });
}

function esc(s) {
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');
}

// Returns "Root › Child › Grandchild" for a given location id
function locationPath(id) {
  const parts = [];
  let current = locations.find(l => l.id === id);
  while (current) {
    parts.unshift(current.name);
    current = current.parent_id ? locations.find(l => l.id === current.parent_id) : null;
  }
  return parts.join(' › ');
}

// Returns flat list sorted by full path, excluding a node and all its descendants
function locationOptionsFor(excludeId = null) {
  const excluded = new Set();
  if (excludeId != null) {
    excluded.add(excludeId);
    let changed = true;
    while (changed) {
      changed = false;
      locations.forEach(l => {
        if (!excluded.has(l.id) && excluded.has(l.parent_id)) {
          excluded.add(l.id);
          changed = true;
        }
      });
    }
  }
  return locations
    .filter(l => !excluded.has(l.id))
    .map(l => ({ id: l.id, path: locationPath(l.id) }))
    .sort((a, b) => a.path.localeCompare(b.path));
}

// ── Detail view ────────────────────────────────────────────────────────────
function openDetailPanel() {
  document.querySelector('main').classList.add('detail-open');
}

function closeDetailPanel() {
  document.querySelector('main').classList.remove('detail-open');
}

async function selectItem(id) {
  if (currentTab === 'locations') {
    selected = locations.find(l => l.id === id);
    if (!selected) return;
    // Expand this node and all its ancestors so it's visible in the tree
    let cur = selected;
    while (cur) {
      expandedIds.add(cur.id);
      cur = cur.parent_id ? locations.find(l => l.id === cur.parent_id) : null;
    }
    renderDetail();
    openDetailPanel();
    const locItems = await api('GET', `/items?location_id=${id}`);
    renderDetail(locItems);
  } else {
    selected = items.find(i => i.id === id);
    if (!selected) return;
    renderDetail();
    openDetailPanel();
  }
  renderList();
}

function renderDetail(locItems = null) {
  const el = document.getElementById('detail');
  if (!selected) { el.innerHTML = '<div class="detail-empty">← select an entry</div>'; return; }

  if (currentTab === 'locations') {
    const parent = locations.find(l => l.id === selected.parent_id);
    const itemsHtml = locItems === null
      ? ''
      : locItems.length === 0
        ? '<p style="color:var(--text-dim);font-size:12px">No items here.</p>'
        : locItems.map(i => `<span class="item-chip" data-item-id="${i.id}">◈ ${esc(i.name)}</span>`).join('');

    el.innerHTML = `<div class="detail-content">
      <button class="btn-back" id="d-back">← back</button>
      <div class="detail-top">
        <div class="detail-title">${esc(selected.name)}</div>
        <div class="detail-actions">
          <button class="btn" id="d-edit">Edit</button>
          <button class="btn danger" id="d-delete">Delete</button>
        </div>
      </div>
      <div class="detail-fields">
        <div class="field">
          <span class="field-label">Parent</span>
          <span class="field-value">${parent ? `<a class="loc-link" data-loc-id="${parent.id}">${esc(locationPath(parent.id))}</a>` : '<span class="muted">— root</span>'}</span>
        </div>
        <div class="field">
          <span class="field-label">Created</span>
          <span class="field-value muted">${fmtDate(selected.created_at)}</span>
        </div>
        <div class="field">
          <span class="field-label">Updated</span>
          <span class="field-value muted">${fmtDate(selected.updated_at)}</span>
        </div>
      </div>
      <div class="quick-create">
        <button class="btn" id="d-new-location">+ Sub-location here</button>
        <button class="btn" id="d-new-item">+ Item here</button>
      </div>
      ${locItems !== null ? `<div class="items-section">
        <div class="section-title">Items here</div>
        <div id="loc-items">${itemsHtml}</div>
      </div>` : ''}
    </div>`;

    el.querySelector('#d-edit').addEventListener('click', () => openForm('edit'));
    el.querySelector('#d-delete').addEventListener('click', deleteSelected);
    el.querySelector('#d-new-location').addEventListener('click', () => openForm('create', 'location', selected.id));
    el.querySelector('#d-new-item').addEventListener('click', () => openForm('create', 'item', selected.id));
    el.querySelectorAll('.loc-link').forEach(a => {
      a.addEventListener('click', () => selectItem(parseInt(a.dataset.locId)));
    });
    el.querySelectorAll('.item-chip').forEach(chip => {
      chip.addEventListener('click', () => {
        currentTab = 'items';
        document.querySelectorAll('.tab').forEach(t => t.classList.toggle('active', t.dataset.tab === 'items'));
        load();
        setTimeout(() => selectItem(parseInt(chip.dataset.itemId)), 100);
      });
    });
  } else {
    const loc = locations.find(l => l.id === selected.location_id);
    el.innerHTML = `<div class="detail-content">
      <button class="btn-back" id="d-back">← back</button>
      <div class="detail-top">
        <div class="detail-title">${esc(selected.name)}</div>
        <div class="detail-actions">
          <button class="btn" id="d-edit">Edit</button>
          <button class="btn danger" id="d-delete">Delete</button>
        </div>
      </div>
      <div class="detail-fields">
        <div class="field">
          <span class="field-label">Location</span>
          <span class="field-value">${loc ? `<a class="loc-link" data-loc-id="${loc.id}">${esc(locationPath(loc.id))}</a>` : '—'}</span>
        </div>
        <div class="field">
          <span class="field-label">Created</span>
          <span class="field-value muted">${fmtDate(selected.created_at)}</span>
        </div>
        <div class="field">
          <span class="field-label">Updated</span>
          <span class="field-value muted">${fmtDate(selected.updated_at)}</span>
        </div>
      </div>
    </div>`;

    el.querySelector('#d-edit').addEventListener('click', () => openForm('edit'));
    el.querySelector('#d-delete').addEventListener('click', deleteSelected);
    el.querySelectorAll('.loc-link').forEach(a => {
      a.addEventListener('click', () => {
        // Switch to locations tab and navigate to that location
        currentTab = 'locations';
        document.querySelectorAll('.tab').forEach(t => t.classList.toggle('active', t.dataset.tab === 'locations'));
        selected = null;
        selectItem(parseInt(a.dataset.locId));
      });
    });
  }

  // Back button (visible only on mobile via CSS)
  const backBtn = el.querySelector('#d-back');
  if (backBtn) {
    backBtn.addEventListener('click', () => {
      selected = null;
      closeDetailPanel();
      renderList();
    });
  }
}

// ── Forms ──────────────────────────────────────────────────────────────────
function openForm(action, entityOverride = null, defaultLocationId = null) {
  const entity = entityOverride || (currentTab === 'locations' ? 'location' : 'item');
  formMode = { action, entity, defaultLocationId };
  const overlay = document.getElementById('form-overlay');
  const title = document.getElementById('form-title');
  const body = document.getElementById('form-body');

  const isEdit = action === 'edit';
  title.textContent = `${isEdit ? 'Edit' : 'New'} ${formMode.entity}`;

  if (formMode.entity === 'location') {
    // When creating with defaultLocationId, pre-select it as parent
    const preParent = isEdit ? selected?.parent_id : defaultLocationId;
    const parentOptions = locationOptionsFor(isEdit ? selected?.id : null)
      .map(o => `<option value="${o.id}"${preParent === o.id ? ' selected' : ''}>${esc(o.path)}</option>`)
      .join('');

    body.innerHTML = `
      <div class="form-group">
        <label class="form-label">Name</label>
        <input class="form-input" id="f-name" type="text" value="${isEdit ? esc(selected.name) : ''}" placeholder="e.g. Living Room">
      </div>
      <div class="form-group">
        <label class="form-label">Parent location</label>
        <select class="form-select" id="f-parent">
          <option value="">— none (root)</option>
          ${parentOptions}
        </select>
      </div>`;
  } else {
    const preLocation = isEdit ? selected?.location_id : defaultLocationId;
    const locOptions = locationOptionsFor()
      .map(o => `<option value="${o.id}"${preLocation === o.id ? ' selected' : ''}>${esc(o.path)}</option>`)
      .join('');

    body.innerHTML = `
      <div class="form-group">
        <label class="form-label">Name</label>
        <input class="form-input" id="f-name" type="text" value="${isEdit ? esc(selected.name) : ''}" placeholder="e.g. HDMI cable">
      </div>
      <div class="form-group">
        <label class="form-label">Location</label>
        <select class="form-select" id="f-location">
          <option value="">— select —</option>
          ${locOptions}
        </select>
      </div>`;
  }

  overlay.classList.add('visible');
  document.getElementById('f-name').focus();
}

function closeForm() {
  document.getElementById('form-overlay').classList.remove('visible');
  formMode = null;
}

async function submitForm() {
  const name = document.getElementById('f-name').value.trim();
  if (!name) { toast('Name is required', true); return; }

  // Remember which location was selected before submit (for quick-create flow)
  const returnToLocation = formMode.action === 'create' && formMode.defaultLocationId
    ? formMode.defaultLocationId
    : null;

  try {
    if (formMode.entity === 'location') {
      const parentVal = document.getElementById('f-parent').value;
      const body = { name, parent_id: parentVal ? parseInt(parentVal) : null };
      if (formMode.action === 'create') {
        await api('POST', '/locations', body);
        toast('Location created');
      } else {
        await api('PUT', `/locations/${selected.id}`, body);
        toast('Location updated');
      }
      await loadLocations(document.getElementById('search').value);
    } else {
      const locVal = document.getElementById('f-location').value;
      if (!locVal) { toast('Location is required', true); return; }
      const body = { name, location_id: parseInt(locVal) };
      if (formMode.action === 'create') {
        await api('POST', '/items', body);
        toast('Item created');
      } else {
        await api('PUT', `/items/${selected.id}`, body);
        toast('Item updated');
      }
      await loadItems(document.getElementById('search').value);
    }
    closeForm();

    if (returnToLocation) {
      // Go back to the parent location view
      currentTab = 'locations';
      document.querySelectorAll('.tab').forEach(t => t.classList.toggle('active', t.dataset.tab === 'locations'));
      await loadLocations();
      await loadItems();
      await selectItem(returnToLocation);
    } else {
      selected = null;
      renderDetail();
    }
  } catch (e) {
    toast(e.message, true);
  }
}

async function deleteSelected() {
  if (!selected) return;
  const label = currentTab === 'locations' ? 'location' : 'item';
  if (!confirm(`Delete "${selected.name}"?`)) return;
  try {
    if (currentTab === 'locations') {
      await api('DELETE', `/locations/${selected.id}`);
      await loadLocations();
    } else {
      await api('DELETE', `/items/${selected.id}`);
      await loadItems();
    }
    selected = null;
    renderDetail();
    toast('Deleted');
  } catch (e) {
    toast(e.message, true);
  }
}

// ── Events ─────────────────────────────────────────────────────────────────
document.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', () => {
    currentTab = tab.dataset.tab;
    document.querySelectorAll('.tab').forEach(t => t.classList.toggle('active', t.dataset.tab === currentTab));
    selected = null;
    document.getElementById('search').value = '';
    closeDetailPanel();
    renderDetail();
    load();
  });
});

document.getElementById('btn-new').addEventListener('click', () => openForm('create'));
document.getElementById('form-close').addEventListener('click', closeForm);
document.getElementById('form-cancel').addEventListener('click', closeForm);
document.getElementById('form-submit').addEventListener('click', submitForm);
document.getElementById('form-overlay').addEventListener('click', e => {
  if (e.target === e.currentTarget) closeForm();
});

document.getElementById('search').addEventListener('input', e => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(() => load(e.target.value.trim()), 250);
});

document.addEventListener('keydown', e => {
  if (e.key === 'Escape') closeForm();
  if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
    document.getElementById('search').focus();
    e.preventDefault();
  }
});

// ── Init ───────────────────────────────────────────────────────────────────
(async () => {
  await loadLocations();
  await loadItems(); // pre-load for lookups
  // reload items list if starting on items tab
  if (currentTab === 'items') renderList();
})();
