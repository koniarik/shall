// The page's script. It reads sets from the server's JSON API and builds the
// document from the answers, which come with every decision about a set
// made: ids resolved, statuses worked out, findings placed. It puts a set's
// text into the page only as text. It keeps the reader's choices about the
// view in the page address, except the colour scheme, which the browser
// keeps.
'use strict';

const body = document.body;
const main = document.getElementById('main');
const outline = document.getElementById('outline');
const notice = document.querySelector('.notice');
const jump = document.querySelector('form.jump');
const menuButton = document.querySelector('button.menu');
const crumb = document.querySelector('.crumb');

// What the page shows: the set's answer, the answers of its unfolded
// chapters, and the version of the files they were built from.
const state = {set: null, chapters: new Map(), version: ''};

// Building elements. Text always goes in as text nodes.

function h(tag, attrs, ...children) {
  const el = document.createElement(tag);
  for (const [name, value] of Object.entries(attrs ?? {})) {
    if (value === undefined || value === null || value === false) continue;
    if (name === 'class') el.className = value;
    else el.setAttribute(name, value === true ? '' : String(value));
  }
  el.append(...children.flat(Infinity).filter((c) => c !== undefined && c !== null && c !== false));
  return el;
}

// safeURL returns a link from a set as an address the page may follow, or
// null.
function safeURL(u) {
  try {
    const url = new URL(u, location.href);
    return ['http:', 'https:', 'mailto:'].includes(url.protocol) ? url.href : null;
  } catch {
    return null;
  }
}

function plural(n, word) {
  return n === 1 ? '1 ' + word : n + ' ' + word + 's';
}

// The page address.

function route() {
  const parts = location.pathname.split('/').filter(Boolean).map(decodeURIComponent);
  return {set: parts[0] ?? '', target: parts[1] ?? ''};
}

function query() {
  return new URLSearchParams(location.search);
}

function address(path, q) {
  const s = q.toString();
  return path + (s ? '?' + s : '');
}

// record keeps in the page address whether value is among name's values.
function record(name, value, on) {
  const q = query();
  const rest = q.getAll(name).filter((v) => v !== value);
  q.delete(name);
  for (const v of rest) q.append(name, v);
  if (on) q.append(name, value);
  history.replaceState(history.state, '', address(location.pathname, q));
}

function setBase(name) {
  return '/' + encodeURIComponent(name) + '/';
}

// The API.

function apiSet(name) {
  return '/api/sets/' + encodeURIComponent(name);
}

function apiFile(name, path) {
  return apiSet(name) + '/files/' + path.split('/').map(encodeURIComponent).join('/');
}

function apiItem(name, id) {
  return apiSet(name) + '/items/' + encodeURIComponent(id);
}

async function api(url) {
  try {
    const r = await fetch(url);
    return {ok: r.ok, body: await r.json()};
  } catch {
    return {ok: false, body: null};
  }
}

// Pieces of the document.

function missionPath() {
  return state.set?.files.find((f) => f.kind === 'mission')?.path;
}

// ref shows what an id names: a link to its item, a retired id or plain text.
function ref(r, withPreview = true) {
  switch (r.state) {
    case 'item': {
      const id = r.itemId ?? r.id;
      const local = r.set === state.set?.name;
      return h('a', {
        'class': 'ref',
        'href': setBase(r.set) + encodeURIComponent(id),
        'data-item': id,
        'data-preview': withPreview ? apiItem(r.set, id) : null,
        'data-chapter': local && r.file !== missionPath() ? r.file : null,
        'data-local': local,
      }, r.id);
    }
    case 'retired':
      return h('span', {class: 'retired'}, h('s', {}, r.id), ' retired',
          r.retiredWith ? ' with ' + r.retiredWith : '', r.title ? ': ' + r.title : '');
    case 'mission':
      return r.id;
    default:
      return h('span', {class: 'unknown'}, r.id);
  }
}

// titled shows what an id names with the item's title beside it.
function titled(r) {
  return r.state === 'item' && r.title ? [ref(r), ' ', h('span', {class: 'ref-title'}, r.title)] : ref(r);
}

function refs(list, show = ref) {
  return list.flatMap((r, i) => (i ? [', ', show(r)] : [show(r)]));
}

// findings shows findings, each with its severity, field and place; withID
// also names the item a finding concerns.
function findings(list, withID = false) {
  if (!list?.length) return null;
  return h('ul', {class: 'findings'}, list.map((f) => {
    const subject = (withID ? [f.id, f.field] : [f.field]).filter(Boolean).join(' ');
    return h('li', {class: 'finding ' + f.severity},
        h('span', {class: 'severity'}, f.severity), ' · ',
        subject ? [h('code', {}, subject), ': '] : null, f.message, ' ',
        h('span', {class: 'where'}, f.line ? f.file + ':' + f.line : f.file));
  }));
}

function status(it) {
  if (it.kind !== 'requirement' && it.kind !== 'need') return null;
  const shown = it.kind === 'requirement' ? it.effectiveStatus || it.status : it.status;
  const written = it.kind === 'requirement' && it.status && it.status !== shown ? it.status : '';
  return h('span', {
    class: 'status',
    title: written ? 'Effective status; written ' + written : null,
  }, shown || 'no status', written ? ' · written ' + written : '');
}

function field(name, ...value) {
  return [h('dt', {}, name), h('dd', {}, ...value)];
}

function list(items) {
  return h('ul', {}, items.map((x) => h('li', {}, x)));
}

function links(list) {
  return h('ul', {}, list.map((l) => {
    const url = safeURL(l.url);
    const label = l.title || l.url;
    return h('li', {}, url ? h('a', {href: url, target: '_blank', rel: 'noreferrer'}, label) : label);
  }));
}

function item(it, level) {
  const requirement = it.kind === 'requirement';
  const fields = [
    it.stakeholder ? field('stakeholder', titled(it.stakeholder)) : null,
    it.measure ? field('measure', it.measure) : null,
    it.verify ? field('verify', it.verify) : null,
    it.mirrors ? field('mirrors', titled(it.mirrors)) : null,
    it.links?.length ? field('links', links(it.links)) : null,
    !requirement && it.notes?.length ? field('notes', list(it.notes)) : null,
  ].filter(Boolean);

  // One fold area: a requirement's rationale and notes, and the items that
  // name the item, grouped by field.
  const folded = requirement ? [
    it.rationale ? field('rationale', it.rationale) : null,
    it.notes?.length ? field('notes', list(it.notes)) : null,
  ].filter(Boolean) : [];
  const named = new Map();
  for (const n of it.namedBy ?? []) {
    if (!named.has(n.field)) named.set(n.field, []);
    named.get(n.field).push(n);
  }
  const summary = [
    [requirement && it.rationale ? 'rationale' : '', requirement && it.notes?.length ? 'notes' : '']
        .filter(Boolean).join(', '),
    ...[...named].map(([name, from]) => ['named in ' + name + ': ', refs(from)]),
  ].filter((part) => part.length);
  const fold = folded.length || named.size ? h('details', {
    'class': 'more', 'data-id': it.id, 'open': query().getAll('details').includes(it.id),
  },
  h('summary', {}, summary.flatMap((part, i) => (i ? [' · ', part] : [part]))),
  h('dl', {class: 'fields'}, folded, [...named].map(([name, from]) =>
    field('named in ' + name, h('ul', {}, from.map((n) => h('li', {}, ref(n), ' ', n.title))))))) : null;

  return h('article', {'class': 'item kind-' + it.kind, 'id': it.id, 'data-anchor': true, 'tabindex': -1},
      h('div', {'class': 'head', 'role': 'heading', 'aria-level': Math.min(level, 6)},
          ref({id: it.id, state: 'item', set: state.set.name, file: it.file}, false), ' ',
          h('span', {class: 'title'}, it.title), ' ', status(it)),
      it.serves?.length ? h('p', {class: 'serves'}, 'serves ', refs(it.serves, titled)) : null,
      h('p', {class: 'text'}, it.text),
      fields.length ? h('dl', {class: 'fields'}, fields) : null,
      findings(it.findings),
      fold,
      it.children?.length ? h('div', {class: 'children'}, it.children.map((c) => item(c, level + 1))) : null);
}

const missionParts = [['Objectives', 'objectives'], ['Stakeholders', 'stakeholders'], ['Needs', 'needs']];

function heading(entry) {
  if (entry.broken) return entry.path;
  return entry.kind === 'mission' ? 'Mission' : entry.title || entry.path;
}

// section shows one file of the set: the mission file in full, a chapter
// folded or with the items of its answer.
function section(entry) {
  const mission = entry.kind === 'mission';
  const full = mission ? entry : state.chapters.get(entry.path);
  const open = !entry.broken && Boolean(full);
  const foldable = !mission && !entry.broken;
  const anchor = 'file:' + entry.path;
  const c = entry.counts;
  let content = null;
  if (open && mission) {
    content = [
      entry.mission ? h('p', {class: 'statement'}, entry.mission) : null,
      missionParts.filter(([, key]) => entry[key]?.length).map(([name, key]) => [
        h('h3', {'id': anchor + ':' + name, 'data-anchor': true}, name),
        entry[key].map((it) => item(it, 4)),
      ]),
    ];
  } else if (open) {
    content = (full.requirements ?? []).map((it) => item(it, 3));
  }
  return h('section', {
    'class': 'file' + (mission ? ' mission' : '') + (entry.broken ? ' broken' : ''),
    'id': anchor, 'data-anchor': true, 'data-path': entry.path, 'data-open': String(open),
    'data-held': foldable && !open && query().getAll('fold').includes(entry.path),
  },
  h('h2', {},
      foldable ? h('button', {
        'type': 'button', 'class': 'fold', 'aria-expanded': String(open), 'aria-controls': 'body:' + entry.path,
      }, heading(entry)) : heading(entry),
      h('span', {class: 'counts'}, plural(c.items, 'item'), c.findings ? ', ' + plural(c.findings, 'finding') : ''),
      h('span', {class: 'path'}, entry.path)),
  open || entry.broken ? findings((full ?? entry).findings) : null,
  h('div', {id: 'body:' + entry.path}, content));
}

function renderOutline() {
  if (!state.set) return;
  const entries = state.set.files.map((f) => {
    const anchor = 'file:' + f.path;
    let sub = [];
    if (f.kind === 'mission' && !f.broken) {
      sub = missionParts.filter(([, key]) => f[key]?.length).map(([name]) => h('li', {class: 'pure-menu-item'},
          h('a', {'class': 'pure-menu-link', 'href': '#' + anchor + ':' + name, 'data-scroll': anchor + ':' + name},
              name)));
    } else if (state.chapters.has(f.path)) {
      sub = (state.chapters.get(f.path).requirements ?? []).map((it) => h('li', {class: 'pure-menu-item'},
          h('a', {
            'class': 'pure-menu-link', 'href': setBase(state.set.name) + encodeURIComponent(it.id),
            'data-item': it.id, 'data-chapter': f.path, 'data-local': true,
          }, h('span', {class: 'ref'}, it.id), ' ', it.title)));
    }
    return h('li', {'class': 'pure-menu-item', 'data-for': anchor},
        h('a', {
          'class': 'pure-menu-link' + (f.broken ? ' broken' : ''), 'href': '#' + anchor, 'data-scroll': anchor,
          'data-path': f.kind === 'chapter' && !f.broken ? f.path : null,
        }, heading(f)),
        sub.length ? h('ul', {class: 'pure-menu-list'}, sub) : null);
  });
  outline.replaceChildren(h('div', {class: 'pure-menu', id: 'outline-menu'},
      h('ul', {class: 'pure-menu-list'}, entries)));
  mark();
}

function renderDocument(missing) {
  main.replaceChildren(
      missing ? h('p', {class: 'missing'}, missing) : '',
      findings(state.set.findings, true) ?? '',
      ...state.set.files.map(section));
  renderOutline();
  watchFolded();
}

function renderStart(answer) {
  main.className = 'alone';
  main.replaceChildren(
      h('h1', {}, 'Sets'),
      findings(answer.findings, true) ?? '',
      answer.clashes.length ? h('ul', {class: 'findings'}, answer.clashes.map((c) => h('li', {class: 'finding error'},
          h('span', {class: 'severity'}, 'error'), ' · Sets ', c.manifests.join(' and '), ' share the name ', c.name,
          ', so neither is served under it.'))) : '',
      answer.sets.length ? h('ul', {class: 'sets'}, answer.sets.map((s) => h('li', {},
          h('a', {href: setBase(s.name)}, s.name), ' ', h('span', {class: 'path'}, s.manifest),
          s.errors ? [' ', h('span', {class: 'count error'}, plural(s.errors, 'error'))] : '',
          s.warnings ? [' ', h('span', {class: 'count warning'}, plural(s.warnings, 'warning'))] : ''))) :
        h('p', {}, 'No set is served here.'));
}

// Loading what the address asks for.

async function loadStart() {
  const r = await api('/api/sets');
  if (!r.ok) return;
  state.version = r.body.version;
  renderStart(r.body);
}

// loadSet fetches the set and the chapters to show unfolded: those the
// address names, those given, and the one holding the address's target.
async function loadSet(also = []) {
  const {set, target} = route();
  const r = await api(apiSet(set));
  if (!r.ok) {
    state.set = null;
    state.version = '';
    main.replaceChildren(h('p', {class: 'missing'}, r.body?.error ?? 'The set cannot be read.'));
    outline.replaceChildren();
    return null;
  }
  let missing = '';
  const want = new Set([...query().getAll('open'), ...also]);
  if (target) {
    const t = await api(apiItem(set, target));
    if (t.ok) want.add(t.body.file);
    else missing = t.body?.error ?? 'No item has the id ' + target + '.';
  }
  const chapters = new Map();
  await Promise.all(r.body.files
      .filter((f) => f.kind === 'chapter' && !f.broken && want.has(f.path))
      .map(async (f) => {
        const c = await api(apiFile(set, f.path));
        if (c.ok) chapters.set(f.path, c.body);
      }));
  state.set = r.body;
  state.chapters = chapters;
  state.version = r.body.version;
  return {missing};
}

async function load() {
  const {set} = route();
  if (!set) {
    await loadStart();
    return;
  }
  body.dataset.base = setBase(set);
  body.classList.add('document');
  document.title = set + ' · shall';
  crumb.replaceChildren(h('span', {class: 'sep'}, ' / '), h('a', {href: setBase(set)}, set));
  jump.hidden = false;
  menuButton.hidden = false;
  outline.hidden = false;
  const loaded = await loadSet();
  if (!loaded) return;
  renderDocument(loaded.missing);
  show(route().target, false);
}

// Folding and unfolding chapters.

// setChapter folds or unfolds a chapter. A choice of the reader's goes into
// the page address; an unfolding on reaching the chapter does not, and keeps
// the reader's place, taken just before the chapter goes in.
async function setChapter(sec, open, {chosen = true, keepPlace = false} = {}) {
  const path = sec.dataset.path;
  if (open) {
    const r = await api(apiFile(state.set.name, path));
    if (!r.ok) {
      sec.dataset.failed = '';
      return;
    }
    state.chapters.set(path, r.body);
  } else {
    state.chapters.delete(path);
  }
  if (chosen) {
    record('open', path, open);
    record('fold', path, !open);
  }
  const entry = state.set.files.find((f) => f.path === path);
  const current = document.getElementById('file:' + path);
  if (!entry || !current) return;
  const focused = current.contains(document.activeElement);
  const place = keepPlace ? anchor() : null;
  current.replaceWith(section(entry));
  if (place) restore(place);
  const t = route().target;
  if (!open && t && !document.getElementById(t)) {
    history.replaceState(history.state, '', address(body.dataset.base, query()));
  }
  if (focused) document.getElementById('file:' + path)?.querySelector('button.fold')?.focus();
  renderOutline();
  watchFolded();
}

// Unfolding chapters as the reader reaches them, one at a time.

function reachable() {
  return [...main.querySelectorAll(
      'section.file[data-open="false"]:not([data-held]):not([data-failed]):not(.broken)')];
}

function inView(sec) {
  const r = sec.querySelector('h2').getBoundingClientRect();
  return r.bottom > 0 && r.top < innerHeight;
}

const reach = new IntersectionObserver(() => unfoldReached());

function watchFolded() {
  reach.disconnect();
  for (const s of reachable()) reach.observe(s.querySelector('h2'));
}

let reaching = false;

async function unfoldReached() {
  if (reaching || !state.set) return;
  reaching = true;
  try {
    for (let next; (next = reachable().find(inView));) {
      await setChapter(next, true, {chosen: false, keepPlace: true});
    }
  } finally {
    reaching = false;
  }
}

// Going places.

// markTarget marks the item the address leads to, and returns it.
function markTarget(id) {
  for (const old of document.querySelectorAll('.target')) old.classList.remove('target');
  const el = id ? document.getElementById(id) : null;
  el?.classList.add('target');
  return el;
}

function show(id, focus = true) {
  const el = markTarget(id);
  if (!el) return;
  el.scrollIntoView({block: 'start'});
  if (focus) el.focus({preventScroll: true});
}

// goTo shows a chapter or a part of the mission, unfolding a folded chapter
// first.
async function goTo(id) {
  const sec = document.getElementById(id);
  if (sec?.dataset.open === 'false' && !sec.classList.contains('broken')) await setChapter(sec, true);
  document.getElementById(id)?.scrollIntoView({block: 'start'});
}

// go follows a link to an item of this set, unfolding its chapter first, and
// leaves the place it left in the history.
async function go(a) {
  const path = a.dataset.chapter;
  const sec = path && document.getElementById('file:' + path);
  if (sec && sec.dataset.open !== 'true') await setChapter(sec, true);
  history.replaceState({...history.state, scroll: scrollY}, '');
  history.pushState(null, '', address(new URL(a.getAttribute('href'), location.href).pathname, query()));
  show(a.dataset.item);
}

document.addEventListener('click', (e) => {
  if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
  const a = e.target.closest('a[data-local]');
  if (a) {
    e.preventDefault();
    hidePreview(true);
    setMenu(false);
    go(a);
    return;
  }
  const entry = e.target.closest('a[data-scroll]');
  if (entry) {
    e.preventDefault();
    setMenu(false);
    goTo(entry.dataset.scroll);
    return;
  }
  const fold = e.target.closest('button.fold');
  if (fold) {
    const sec = fold.closest('section.file');
    setChapter(sec, sec.dataset.open !== 'true');
  }
});

// Unfolding an item's fold area is a choice about the view, which the
// address keeps.
document.addEventListener('toggle', (e) => {
  if (e.target.matches?.('details.more')) record('details', e.target.dataset.id, e.target.open);
}, true);

// The outline marks the chapter at the top of the screen and keeps its entry
// in sight.

function mark() {
  const edge = (document.querySelector('.bar')?.getBoundingClientRect().bottom ?? 0) + 8;
  const sections = [...main.querySelectorAll('section.file')];
  const current = sections.findLast((s) => s.getBoundingClientRect().top <= edge) ?? sections[0];
  for (const li of outline.querySelectorAll('#outline-menu > ul > li')) {
    const on = li.dataset.for === current?.id;
    li.classList.toggle('pure-menu-selected', on);
    const a = li.querySelector(':scope > a');
    if (on) {
      a.setAttribute('aria-current', 'location');
      const r = a.getBoundingClientRect();
      const n = outline.getBoundingClientRect();
      if (r.top < n.top || r.bottom > n.bottom) outline.scrollTop += r.top - n.top - n.height / 3;
    } else {
      a.removeAttribute('aria-current');
    }
  }
}

let marking = false;
addEventListener('scroll', () => {
  if (marking) return;
  marking = true;
  requestAnimationFrame(() => {
    marking = false;
    mark();
  });
}, {passive: true});

// On a narrow screen the outline folds behind a button.

function setMenu(open) {
  if (menuButton.hidden) return;
  body.classList.toggle('outline-open', open);
  menuButton.setAttribute('aria-expanded', String(open));
}

menuButton.addEventListener('click', () => setMenu(!body.classList.contains('outline-open')));

// The colour scheme: the system's, until the reader chooses one, which the
// browser keeps for every page.

const root = document.documentElement;
const dark = matchMedia('(prefers-color-scheme: dark)');

function scheme() {
  return root.dataset.theme || (dark.matches ? 'dark' : 'light');
}

document.querySelector('button.scheme').addEventListener('click', () => {
  const next = scheme() === 'dark' ? 'light' : 'dark';
  root.dataset.theme = next;
  try {
    localStorage.setItem('shall-scheme', next);
  } catch {
    // Without storage, the choice lasts as long as the page.
  }
});

addEventListener('storage', (e) => {
  if (e.key === 'shall-scheme' && (e.newValue === 'dark' || e.newValue === 'light')) {
    root.dataset.theme = e.newValue;
  }
});

// Finding: the server says what a reader typed names, an item by its id or
// the items that hold the words, and the page lists them under the box.

const input = jump.elements.id;
const results = document.getElementById('results');
let found = [];
let active = -1;
let searches = 0;
let searchTimer = 0;

async function search(q) {
  const n = ++searches;
  const r = await api(apiSet(state.set.name) + '/search?q=' + encodeURIComponent(q));
  return n === searches && r.ok ? r.body : null;
}

function listFound(list) {
  found = list;
  active = -1;
  results.replaceChildren(...list.map((r, i) => h('li', {
    'id': 'result-' + i, 'role': 'option', 'aria-selected': 'false', 'data-index': i,
  }, h('span', {class: 'ref'}, r.itemId ?? r.id), ' ', r.title)));
  results.hidden = list.length === 0;
  input.setAttribute('aria-expanded', String(!results.hidden));
  input.removeAttribute('aria-activedescendant');
}

function choose(i) {
  active = i;
  for (const li of results.children) li.setAttribute('aria-selected', String(Number(li.dataset.index) === i));
  input.setAttribute('aria-activedescendant', 'result-' + i);
  results.children[i].scrollIntoView({block: 'nearest'});
}

function goFound(r) {
  listFound([]);
  notice.textContent = '';
  const link = ref(r);
  if (link.hasAttribute('data-local')) go(link);
  else location.assign(link.getAttribute('href'));
}

input.addEventListener('input', () => {
  notice.textContent = '';
  clearTimeout(searchTimer);
  const q = input.value.trim();
  if (q.length < 2 || !state.set) {
    searches++;
    listFound([]);
    return;
  }
  searchTimer = setTimeout(async () => {
    const a = await search(q);
    if (a) listFound(a.results);
  }, 150);
});

input.addEventListener('keydown', (e) => {
  if (e.key === 'ArrowDown' && found.length) {
    e.preventDefault();
    choose(Math.min(active + 1, found.length - 1));
  } else if (e.key === 'ArrowUp' && found.length) {
    e.preventDefault();
    choose(Math.max(active - 1, 0));
  } else if (e.key === 'Escape' && found.length) {
    e.stopPropagation();
    listFound([]);
  }
});

input.addEventListener('blur', () => listFound([]));

results.addEventListener('mousedown', (e) => {
  const li = e.target.closest('li[data-index]');
  if (!li) return;
  e.preventDefault();
  goFound(found[Number(li.dataset.index)]);
});

jump.addEventListener('submit', async (e) => {
  e.preventDefault();
  if (active >= 0) {
    goFound(found[active]);
    return;
  }
  const q = input.value.trim();
  if (!q || !state.set) return;
  clearTimeout(searchTimer);
  const a = await search(q);
  if (!a) return;
  if (a.results.length) goFound(a.results[0]);
  else notice.textContent = a.message ?? '';
});

// Previews, on pointing at a link or moving keyboard focus to it.

const tip = h('div', {id: 'preview', role: 'tooltip', hidden: true});
body.append(tip);
const previews = new Map();
let shown = null;
let hideTimer = 0;

async function showPreview(a) {
  clearTimeout(hideTimer);
  if (shown === a) return;
  shown?.removeAttribute('aria-describedby');
  shown = a;
  const url = a.dataset.preview;
  if (!previews.has(url)) {
    const r = await api(url);
    previews.set(url, r.ok ? r.body : null);
  }
  const it = previews.get(url);
  if (shown !== a || !it) return;
  tip.replaceChildren(h('div', {class: 'summary'},
      h('b', {}, it.id + ' ' + it.title), ' ', status(it), h('p', {}, it.text)));
  tip.hidden = false;
  const r = a.getBoundingClientRect();
  const left = Math.max(8, Math.min(r.left, innerWidth - tip.offsetWidth - 8));
  tip.style.left = (scrollX + left) + 'px';
  tip.style.top = (scrollY + r.bottom + 6) + 'px';
  a.setAttribute('aria-describedby', 'preview');
}

function hidePreview(now) {
  clearTimeout(hideTimer);
  hideTimer = setTimeout(() => {
    shown?.removeAttribute('aria-describedby');
    shown = null;
    tip.hidden = true;
  }, now ? 0 : 300);
}

document.addEventListener('mouseover', (e) => {
  const a = e.target.closest('a[data-preview]');
  if (a) showPreview(a);
  else if (e.target.closest('#preview')) clearTimeout(hideTimer);
});
document.addEventListener('mouseout', (e) => {
  if (e.target.closest('a[data-preview], #preview')) hidePreview(false);
});
document.addEventListener('focusin', (e) => {
  const a = e.target.closest('a[data-preview]');
  if (a) showPreview(a);
});
document.addEventListener('focusout', (e) => {
  if (e.target.closest('a[data-preview]')) hidePreview(true);
});
document.addEventListener('keydown', (e) => {
  if (e.key !== 'Escape') return;
  hidePreview(true);
  setMenu(false);
});

// Following the files.

// anchor returns the first item or heading at the top of the screen, the
// ones after it, and the chapter that holds it, with where the first stands.
function anchor() {
  const els = [...main.querySelectorAll('[data-anchor]')];
  const i = els.findIndex((el) => el.getBoundingClientRect().top >= 0);
  if (i < 0) return null;
  const ids = els.slice(i).map((el) => el.id);
  const holder = els[i].closest('section.file');
  if (holder) ids.push(holder.id);
  return {ids, top: els[i].getBoundingClientRect().top};
}

function restore(place) {
  for (const id of place.ids) {
    const el = document.getElementById(id);
    if (el) {
      scrollBy(0, el.getBoundingClientRect().top - place.top);
      return;
    }
  }
}

let refreshing = false;
// pending is the kind of refresh asked for while one runs: true to keep the
// place, false to go where the address leads, which wins.
let pending = null;

// refresh fetches what the page shows again and puts it in place, keeping
// the reader's place, or going where the address leads.
async function refresh(keepPlace) {
  if (refreshing) {
    pending = pending === false ? false : keepPlace;
    return;
  }
  refreshing = true;
  try {
    for (let mode = keepPlace; mode !== null; mode = pending, pending = null) {
      if (!route().set) {
        await loadStart();
        continue;
      }
      const loaded = await loadSet(mode ? [...state.chapters.keys()] : []);
      if (!loaded) continue;
      const place = mode ? anchor() : null;
      previews.clear();
      hidePreview(true);
      renderDocument(loaded.missing);
      if (place) {
        markTarget(route().target);
        restore(place);
      } else if (history.state?.scroll !== undefined) {
        markTarget(route().target);
        scrollTo(0, history.state.scroll);
      } else {
        show(route().target, false);
      }
    }
  } finally {
    refreshing = false;
  }
}

// A hidden page lets go of its stream, and catches up when shown again: the
// server sends the current version as soon as a stream opens.
let events = null;

function listen() {
  if (events) return;
  events = new EventSource('/api/events');
  events.addEventListener('version', (e) => {
    if (e.data !== state.version) refresh(true);
  });
}

function letGo() {
  events?.close();
  events = null;
}

document.addEventListener('visibilitychange', () => document.hidden ? letGo() : listen());
addEventListener('pagehide', letGo);
addEventListener('popstate', () => refresh(false));
history.scrollRestoration = 'manual';

load().then(() => {
  if (!document.hidden) listen();
});
