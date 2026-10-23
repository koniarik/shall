// The page's script. It reads sets from the server's JSON API and builds the
// document from the answers, which come with every decision about a set
// made: ids resolved, statuses worked out, findings placed. It puts a set's
// text into the page only as text. It keeps the reader's choices about the
// view in the page address, except the colour scheme, which the browser
// keeps.
//
// The same script reads a static copy, which shall export writes: there the
// answers are files, nothing changes while the page is open, and search
// runs in the page.
'use strict';

// Where the page is served from, and whether it reads a static copy.
const base = document.querySelector('meta[name="shall-base"]')?.content || '/';
const copy = Boolean(document.querySelector('meta[name="shall-static"]'));

const body = document.body;
const main = document.getElementById('main');
const outline = document.getElementById('outline');
const notice = document.querySelector('.notice');
const jump = document.querySelector('form.jump');
const menuButton = document.querySelector('button.menu');
const crumb = document.querySelector('.crumb');
const barFilter = document.querySelector('.bar .filter');

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

// route reads the set and the target from the page address. An address of
// a static copy may name the page's file: index.html, or the target with
// .html added.
function route() {
  const path = location.pathname.startsWith(base) ? location.pathname.slice(base.length) : location.pathname;
  const parts = path.split('/').filter(Boolean).map((p) => {
    try {
      return decodeURIComponent(p);
    } catch {
      return p;
    }
  });
  if (parts.at(-1) === 'index.html') parts.pop();
  return {set: parts[0] ?? '', target: (parts[1] ?? '').replace(/\.html$/, '')};
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
  return base + encodeURIComponent(name) + '/';
}

// The API. A resource is named by its path within the API, which apiURL
// turns into an address: the server's, or a file of a static copy.

function apiSet(name) {
  return 'sets/' + encodeURIComponent(name);
}

function apiFile(name, path) {
  return apiSet(name) + '/files/' + path.split('/').map(encodeURIComponent).join('/');
}

function apiItem(name, id) {
  return apiSet(name) + '/items/' + encodeURIComponent(id);
}

function apiURL(path) {
  return base + 'api/' + path + (copy ? '.json' : '');
}

// api fetches a resource; an answer that says what is missing is not ok,
// which a static copy can only say in its body.
async function api(path) {
  try {
    const r = await fetch(apiURL(path));
    const body = await r.json();
    return {ok: r.ok && !body?.error, body};
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

// A topic's text comes as blocks of text, which the page builds as text.

function blocks(list, level) {
  return list.map((b) => block(b, level));
}

function block(b, level) {
  switch (b.type) {
    case 'heading':
      return h('div', {'class': 'md-heading', 'role': 'heading', 'aria-level': Math.min(level + b.level, 6)},
          inline(b.inline));
    case 'list':
      return h(b.ordered ? 'ol' : 'ul', {start: b.ordered && b.start !== 1 ? b.start : null},
          b.items.map((it) => h('li', {}, blocks(it, level))));
    case 'quote':
      return h('blockquote', {}, blocks(b.blocks, level));
    case 'code':
      return h('pre', {}, h('code', {}, b.text));
    case 'html':
      return h('pre', {class: 'html'}, h('code', {}, b.text));
    case 'rule':
      return h('hr', {});
    case 'table':
      return h('table', {class: 'md-table'},
          h('thead', {}, h('tr', {}, b.head.map((c, i) => h('th', {class: b.align?.[i] ? 'align-' + b.align[i] : null}, inline(c))))),
          h('tbody', {}, b.rows.map((row) => h('tr', {},
              row.map((c, i) => h('td', {class: b.align?.[i] ? 'align-' + b.align[i] : null}, inline(c)))))));
    case 'diagram':
      return b.language === 'dot' ? dotDiagram(b) : diagram(b.text);
    default:
      return h('p', {}, inline(b.inline ?? []));
  }
}

function inline(list) {
  return list.map((x) => {
    switch (x.type) {
      case 'emphasis':
        return h('em', {}, inline(x.inline));
      case 'strong':
        return h('strong', {}, inline(x.inline));
      case 'code':
        return h('code', {}, x.text);
      case 'break':
        return h('br', {});
      case 'ref':
        return ref(x.ref);
      case 'link': {
        const url = safeURL(x.url);
        return url ? h('a', {href: url, title: x.title || null, target: '_blank', rel: 'noreferrer'}, inline(x.inline)) :
          inline(x.inline);
      }
      default:
        return x.text;
    }
  });
}

// Diagrams: Mermaid draws each, loaded the first time the page shows one.
// Its drawing is the one markup the page puts in, made at Mermaid's strict
// level, which runs no script in a diagram, and with labels drawn as text
// rather than HTML.

let mermaidLoaded = null;
let drawing = Promise.resolve();
let drawn = 0;

function loadMermaid() {
  mermaidLoaded ??= new Promise((resolve, reject) => {
    const s = h('script', {src: base + '-/static/mermaid/mermaid.min.js'});
    s.addEventListener('load', () => resolve(window.mermaid));
    s.addEventListener('error', () => reject(new Error('Mermaid did not load')));
    document.head.append(s);
  });
  return mermaidLoaded;
}

// diagram shows a diagram's source until Mermaid has drawn it.
function diagram(source) {
  const el = h('figure', {'class': 'diagram', 'data-source': source}, h('pre', {}, h('code', {}, source)));
  draw(el);
  return el;
}

// draw draws a diagram in the colour scheme shown, one diagram at a time; a
// diagram Mermaid cannot draw shows its source and why.
function draw(el) {
  drawing = drawing.then(async () => {
    try {
      const mermaid = await loadMermaid();
      mermaid.initialize({
        startOnLoad: false, securityLevel: 'strict', htmlLabels: false, flowchart: {htmlLabels: false},
        theme: scheme() === 'dark' ? 'dark' : 'neutral',
      });
      const {svg} = await mermaid.render('diagram-' + ++drawn, el.dataset.source);
      el.innerHTML = svg;
      el.dataset.drawn = 'true';
    } catch (e) {
      el.replaceChildren(h('pre', {}, h('code', {}, el.dataset.source)), h('p', {class: 'finding error'},
          h('span', {class: 'severity'}, 'error'), ' · the diagram cannot be drawn: ', String(e?.message ?? e)));
      el.dataset.drawn = 'false';
    }
  });
}

function redraw() {
  for (const el of main.querySelectorAll('figure.diagram:not(.dot)')) draw(el);
}

// dotDiagram shows a DOT diagram as the server drew it, the SVG it cleaned put
// in as markup, or its source and why it cannot be drawn.
function dotDiagram(b) {
  if (!b.svg) {
    return h('figure', {'class': 'diagram dot', 'data-drawn': 'false'}, h('pre', {}, h('code', {}, b.text)),
        h('p', {class: 'finding error'}, h('span', {class: 'severity'}, 'error'), ' · the diagram cannot be drawn: ', b.error ?? ''));
  }
  const svg = new DOMParser().parseFromString(b.svg, 'image/svg+xml').documentElement;
  return h('figure', {'class': 'diagram dot', 'data-drawn': 'true'}, document.importNode(svg, true));
}

function item(it, level) {
  const requirement = it.kind === 'requirement';
  const cites = it.cites ?? {};
  const notes = (it.notes ?? []).map((n, i) => (cites.notes?.[i] ? inline(cites.notes[i]) : n));
  const fields = [
    it.stakeholders?.length ? field('stakeholders', refs(it.stakeholders, titled)) : null,
    it.measure ? field('measure', cites.measure ? inline(cites.measure) : it.measure) : null,
    it.verify ? field('verify', it.verify) : null,
    it.mirrors ? field('mirrors', titled(it.mirrors)) : null,
    it.links?.length ? field('links', links(it.links)) : null,
    !requirement && notes.length ? field('notes', list(notes)) : null,
  ].filter(Boolean);

  // One fold area: a requirement's rationale and notes, and the items that
  // name or cite the item, each once, in reading order.
  const folded = requirement ? [
    it.rationale ? field('rationale', cites.rationale ? inline(cites.rationale) : it.rationale) : null,
    notes.length ? field('notes', list(notes)) : null,
  ].filter(Boolean) : [];
  const named = [];
  for (const n of it.namedBy ?? []) {
    if (!named.some((m) => m.id === n.id)) named.push(n);
  }
  const summary = [
    [requirement && it.rationale ? 'rationale' : '', requirement && notes.length ? 'notes' : '']
        .filter(Boolean).join(', '),
    named.length ? ['named by ', refs(named)] : '',
  ].filter((part) => part.length);
  const fold = folded.length || named.length ? h('details', {
    'class': 'more', 'data-id': it.id, 'open': query().getAll('details').includes(it.id),
  },
  h('summary', {}, summary.flatMap((part, i) => (i ? [' · ', part] : [part]))),
  h('dl', {class: 'fields'}, folded, named.length ?
    field('named by', h('ul', {}, named.map((n) => h('li', {}, ref(n), ' ', n.title)))) : null)) : null;

  return h('article', {
    'class': 'item kind-' + it.kind + (it.context ? ' context' : ''),
    'id': it.id, 'data-anchor': true, 'tabindex': -1,
  },
      h('div', {'class': 'head', 'role': 'heading', 'aria-level': Math.min(level, 6)},
          ref({id: it.id, state: 'item', set: state.set.name, file: it.file}, false), ' ',
          h('span', {class: 'title'}, it.title), ' ', status(it),
          it.requirements?.total ? h('span', {class: 'counts'}, statuses(it.requirements)) : null),
      it.serves?.length ? h('p', {class: 'serves'}, 'serves ', refs(it.serves, titled)) : null,
      it.blocks ? h('div', {class: 'text'}, blocks(it.blocks, level)) :
        it.text ? h('p', {class: 'text'}, cites.text ? inline(cites.text) : it.text) : null,
      fields.length ? h('dl', {class: 'fields'}, fields) : null,
      findings(it.findings),
      fold,
      it.children?.length ? h('div', {class: 'children'}, it.children.map((c) => item(c, level + 1))) : null);
}

// byStatus returns items as the status filter shows them, when it chooses any
// statuses: of the requirements, those at a chosen effective status, with
// the requirements and topics above them as context; items of the other
// kinds with what is shown below them.
function byStatus(items, chosen) {
  if (!chosen.length) return items;
  const out = [];
  for (const it of items) {
    const children = byStatus(it.children ?? [], chosen);
    const kept = {...it, children, context: false};
    if (it.kind === 'requirement') {
      const at = chosen.includes(it.effectiveStatus);
      if (!at && !children.length) continue;
      kept.context = !at;
    } else if (it.kind === 'topic') {
      if (!children.length) continue;
      kept.context = true;
    }
    out.push(kept);
  }
  return out;
}

function heading(entry) {
  if (entry.broken) return entry.path;
  return entry.title || (entry.kind === 'mission' ? 'Mission' : entry.path);
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
  const chosen = query().getAll('status');
  const matching = matches(entry);
  const empty = foldable && emptyUnderFilter(entry);
  let content = null;
  if (open) {
    content = [
      mission && entry.mission ? h('p', {class: 'statement'},
          entry.cites?.mission ? inline(entry.cites.mission) : entry.mission) : null,
      byStatus(full.items ?? [], chosen).map((it) => item(it, 3)),
    ];
  }
  return h('section', {
    'class': 'file' + (mission ? ' mission' : '') + (entry.broken ? ' broken' : ''),
    'id': anchor, 'data-anchor': true, 'data-path': entry.path, 'data-open': String(open),
    'data-held': foldable && !open && query().getAll('fold').includes(entry.path),
    'data-empty': empty,
  },
  h('h2', {},
      foldable ? h('button', {
        'type': 'button', 'class': 'fold', 'aria-expanded': String(open), 'aria-controls': 'body:' + entry.path,
      }, heading(entry)) : heading(entry),
      h('span', {class: 'counts'}, plural(c.items, 'item'), c.findings ? ', ' + plural(c.findings, 'finding') : '',
          foldable && chosen.length ? ' · ' + matching + ' at ' + chosen.join(' or ') : ''),
      h('span', {class: 'path'}, entry.path)),
  open || entry.broken ? findings((full ?? entry).findings) : null,
  h('div', {id: 'body:' + entry.path}, content));
}

// outlined lists items in the outline, each topic with the topics it holds
// under it.
function outlined(items, path) {
  return items.map((it) => {
    const topics = it.kind === 'topic' ? (it.children ?? []).filter((c) => c.kind === 'topic') : [];
    return h('li', {class: 'pure-menu-item'},
        h('a', {
          'class': 'pure-menu-link', 'href': setBase(state.set.name) + encodeURIComponent(it.id),
          'data-item': it.id, 'data-chapter': path, 'data-local': true,
        }, h('span', {class: 'ref'}, it.id), ' ', it.title),
        topics.length ? h('ul', {class: 'pure-menu-list'}, outlined(topics, path)) : null);
  });
}

function renderOutline() {
  if (!state.set) return;
  const entries = state.set.files.map((f) => {
    const anchor = 'file:' + f.path;
    const full = f.kind === 'mission' ? f : state.chapters.get(f.path);
    const sub = full && !f.broken ? outlined(byStatus(full.items ?? [], query().getAll('status')), f.path) : [];
    return h('li', {'class': 'pure-menu-item', 'data-for': anchor},
        h('a', {
          'class': 'pure-menu-link' + (f.broken ? ' broken' : ''), 'href': '#' + anchor, 'data-scroll': anchor,
          'data-path': f.kind === 'chapter' && !f.broken ? f.path : null,
        }, heading(f)),
        sub.length ? h('ul', {class: 'pure-menu-list'}, sub) : null);
  });
  const buttons = filterButtons();
  outline.replaceChildren(
      buttons.length ? h('div', {
        'class': 'filter', 'role': 'group', 'aria-label': 'Show only the requirements at a status',
      }, buttons) : '',
      h('div', {class: 'pure-menu', id: 'outline-menu'}, h('ul', {class: 'pure-menu-list'}, entries)));
  mark();
}

const statusNames = ['draft', 'approved', 'implemented', 'verified', 'waived'];

// matches returns how many of a file's requirements stand at a status the
// address chooses.
function matches(entry) {
  return query().getAll('status').reduce((n, s) => n + (entry.counts.requirements[s] ?? 0), 0);
}

// emptyUnderFilter tells whether a chapter holds nothing the chosen statuses
// show.
function emptyUnderFilter(entry) {
  return entry.kind === 'chapter' && query().getAll('status').length > 0 && matches(entry) === 0;
}

// filterButtons offers the effective statuses the set's requirements stand
// at, each with its count, to show only the requirements at the chosen ones.
// The bar holds them, or on a narrow screen the outline.
function filterButtons() {
  const counts = state.set.requirements;
  const chosen = query().getAll('status');
  return statusNames.filter((s) => counts[s] || chosen.includes(s)).map((s) => h('button', {
    'type': 'button', 'class': 'pure-button status-filter', 'data-status': s,
    'aria-pressed': String(chosen.includes(s)),
  }, s + ' ' + (counts[s] ?? 0)));
}

// dropFilter shows every requirement again.
function dropFilter() {
  for (const s of query().getAll('status')) record('status', s, false);
}

function renderDocument(missing) {
  const buttons = filterButtons();
  barFilter.replaceChildren(...buttons);
  barFilter.hidden = buttons.length === 0;
  // On a narrow screen the filter sits behind the Outline button, which says
  // when one is on.
  const filtered = query().getAll('status').length > 0;
  menuButton.textContent = filtered ? 'Outline •' : 'Outline';
  menuButton.setAttribute('aria-label', filtered ? 'Outline, filtered by status' : 'Outline');
  main.replaceChildren(
      missing ? h('p', {class: 'missing'}, missing) : '',
      findings(state.set.findings, true) ?? '',
      ...state.set.files.map(section));
  renderOutline();
  watchFolded();
}

// statuses shows how many requirements stand at each effective status.
function statuses(c) {
  const parts = [plural(c.total, 'requirement')];
  for (const name of ['draft', 'approved', 'implemented', 'verified', 'waived', 'other']) {
    if (c[name]) parts.push(c[name] + ' ' + name);
  }
  return parts.join(' · ');
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
          s.warnings ? [' ', h('span', {class: 'count warning'}, plural(s.warnings, 'warning'))] : '',
          h('div', {class: 'statuses'}, statuses(s.requirements))))) :
        h('p', {}, 'No set is served here.'));
}

// Loading what the address asks for.

async function loadStart() {
  const r = await api('sets');
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
    // A static host answers what a copy lacks with its own page, not JSON.
    const lacking = copy ? 'No set named ' + set + ' is served here.' : 'The set cannot be read.';
    main.replaceChildren(h('p', {class: 'missing'}, r.body?.error ?? lacking));
    outline.replaceChildren();
    return null;
  }
  let missing = '';
  const want = new Set([...query().getAll('open'), ...also]);
  if (target) {
    const t = await api(apiItem(set, target));
    if (t.ok) want.add(t.body.file);
    else missing = t.body?.error ?? 'No item of ' + set + ' has the id ' + target + '.';
  }
  const chapters = new Map();
  await Promise.all(r.body.files
      .filter((f) => f.kind === 'chapter' && !f.broken && want.has(f.path) && !emptyUnderFilter(f))
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
  let loaded = await loadSet();
  if (!loaded) return;
  renderDocument(loaded.missing);
  const t = route().target;
  if (t && !document.getElementById(t) && query().getAll('status').length) {
    dropFilter();
    loaded = await loadSet();
    if (!loaded) return;
    renderDocument(loaded.missing);
  }
  show(t, false);
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
      'section.file[data-open="false"]:not([data-held]):not([data-failed]):not([data-empty]):not(.broken)')];
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

// goTo shows a chapter, unfolding it first when it is folded.
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
  if (!document.getElementById(a.dataset.item) && query().getAll('status').length) {
    dropFilter();
    await refresh(true);
  }
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
    return;
  }
  const status = e.target.closest('button.status-filter');
  if (status) {
    record('status', status.dataset.status, status.getAttribute('aria-pressed') !== 'true');
    refresh(true);
  }
});

// Unfolding an item's fold area is a choice about the view, which the
// address keeps.
document.addEventListener('toggle', (e) => {
  if (e.target.matches?.('details.more')) record('details', e.target.dataset.id, e.target.open);
}, true);

// The outline marks the chapter at the top of the screen, and the part of it
// the reader is in: one of its top-level items. It keeps the marked entry in
// sight.

// lastAbove returns the last of els whose top is above edge.
function lastAbove(els, edge) {
  return els.findLast((el) => el.getBoundingClientRect().top <= edge);
}

function markEntry(li, on) {
  li.classList.toggle('pure-menu-selected', on);
  const a = li.querySelector(':scope > a');
  if (on) a.setAttribute('aria-current', 'location');
  else a.removeAttribute('aria-current');
}

function mark() {
  const edge = (document.querySelector('.bar')?.getBoundingClientRect().bottom ?? 0) + 8;
  const sections = [...main.querySelectorAll('section.file')];
  const current = lastAbove(sections, edge) ?? sections[0];
  const part = current && lastAbove([...current.querySelectorAll(':scope > div > article.item')], edge);
  let shown = null;
  for (const li of outline.querySelectorAll('#outline-menu > ul > li')) {
    const on = li.dataset.for === current?.id;
    markEntry(li, on);
    if (on) shown = li.querySelector(':scope > a');
    for (const sub of li.querySelectorAll(':scope > ul > li')) {
      const a = sub.querySelector(':scope > a');
      const subOn = on && Boolean(part) && a.dataset.item === part.id;
      markEntry(sub, subOn);
      if (subOn) shown = a;
    }
  }
  if (shown) {
    const r = shown.getBoundingClientRect();
    const n = outline.getBoundingClientRect();
    if (r.top < n.top || r.bottom > n.bottom) outline.scrollTop += r.top - n.top - n.height / 3;
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

dark.addEventListener('change', () => {
  if (!root.dataset.theme) redraw();
});

document.querySelector('button.scheme').addEventListener('click', () => {
  const next = scheme() === 'dark' ? 'light' : 'dark';
  root.dataset.theme = next;
  redraw();
  try {
    localStorage.setItem('shall-scheme', next);
  } catch {
    // Without storage, the choice lasts as long as the page.
  }
});

addEventListener('storage', (e) => {
  if (e.key === 'shall-scheme' && (e.newValue === 'dark' || e.newValue === 'light')) {
    root.dataset.theme = e.newValue;
    redraw();
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
  let a = null;
  if (copy) {
    a = await searchCopy(q);
  } else {
    const r = await api(apiSet(state.set.name) + '/search?q=' + encodeURIComponent(q));
    if (r.ok) a = r.body;
  }
  return n === searches ? a : null;
}

// A static copy has no server to search, so the page finds what the
// server's search would: the item a single word names as an id, then the
// items whose id and title hold every word, then those whose text does,
// each in reading order and ignoring case. Words are split, trimmed and
// lowercased by the rules of Go's strings package.

const idLike = /^[OSNRT][0-9]+(\.[0-9]+)*$/i;
const searchLimit = 50;
const copySets = new Map();

// The white space of Go's unicode.IsSpace.
const goSpace = '[\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000]';
const goSpaces = new RegExp(goSpace + '+');
const goEdges = new RegExp('^' + goSpace + '+|' + goSpace + '+$', 'g');

// goLower lowercases as Go's strings.ToLower: each code point by its own
// mapping, without the special cases of toLowerCase.
function goLower(s) {
  let out = '';
  for (const c of s) out += c === '\u0130' ? 'i' : c.toLowerCase();
  return out;
}

async function searchCopy(typed) {
  const q = typed.replace(goEdges, '');
  const a = {query: q, results: []};
  const words = goLower(q).split(goSpaces).filter(Boolean);
  if (!words.length || !state.set) return a;
  const name = state.set.name;
  const set = await copySet(name);
  let exact = null;
  let message = '';
  let retired = false;
  if (words.length === 1) ({exact, message, retired} = await resolveCopy(q));
  if (exact) a.results.push({match: 'id', ...exact});
  const holds = (s) => {
    const lower = goLower(s);
    return words.every((w) => lower.includes(w));
  };
  const groups = [[], []];
  for (const it of set?.items ?? []) {
    if (exact && exact.set === name && it.id === (exact.itemId ?? exact.id)) continue;
    if (holds(it.id + ' ' + it.title)) groups[0].push(it);
    else if (holds(it.id + ' ' + it.title + ' ' + it.text)) groups[1].push(it);
  }
  ['title', 'text'].forEach((match, i) => {
    for (const it of groups[i]) {
      if (a.results.length === searchLimit) {
        a.more = (a.more ?? 0) + 1;
        continue;
      }
      a.results.push({match, ...refCopy(name, set, it.id, it.id)});
    }
  });
  if (!a.results.length) {
    const why = retired || q.includes('/') || idLike.test(q) ? message : 'No item holds ' + words.join(', ') + '.';
    if (why) a.message = why;
  }
  return a;
}

// resolveCopy says what an id names, as the reader typed it: an id of the
// set, or name/id of another, where name is one the manifest gives a set
// under depends or the name of a served set; matched exactly or else in
// capitals.
async function resolveCopy(typed) {
  let name = state.set.name;
  let local = typed;
  const slash = typed.indexOf('/');
  if (slash >= 0) {
    const given = typed.slice(0, slash);
    const depends = state.set.depends ?? {};
    local = typed.slice(slash + 1);
    name = Object.hasOwn(depends, given) ? depends[given] : given;
    const sets = name === given ? await api('sets') : null;
    if (sets && !sets.body?.sets?.some((s) => s.name === name)) {
      return {exact: null, message: 'No set named ' + given + ' is served here.'};
    }
  }
  const set = await copySet(name);
  if (!set?.item(local) && !set?.retiredWith(local)) local = local.toUpperCase();
  const r = refCopy(name, set, local, typed);
  if (r.state === 'retired') {
    const e = set.retiredWith(local);
    return {exact: null, retired: true, message: typed + ' was retired at ' + e.file + ':' + e.line + '.'};
  }
  if (r.state === 'unknown') return {exact: null, message: 'No item has the id ' + typed + '.'};
  return {exact: r, message: ''};
}

// refCopy returns what id names in a set of the copy, written as written,
// as the server's refs say it: retired when its path or an ancestor's is,
// else the item at its path when that has the id, else unknown.
function refCopy(name, set, id, written) {
  const r = {id: written, state: 'unknown'};
  const e = set?.retiredWith(id);
  const it = e ? null : set?.item(id);
  if (e) {
    Object.assign(r, {state: 'retired', set: name, title: e.title});
    if (e.id !== id) r.retiredWith = e.id;
  } else if (it) {
    Object.assign(r, {state: 'item', set: name, title: it.title, file: it.file});
  }
  if (r.state !== 'unknown' && id !== written) r.itemId = id;
  return r;
}

// idPath returns the path of an id, the id without its letter, as the
// server keys items and retired entries.
function idPath(id) {
  return id.charCodeAt(0) < 128 ? id.slice(1) : '\0' + id;
}

// copySet returns a set of the copy as its search reads it: its items in
// reading order, the first at each path not retired; the item an id names;
// and the entry that retired an id's path or an ancestor's, whatever the
// letter. It reads each chapter's file once, and again after a read failed.
function copySet(name) {
  if (!copySets.has(name)) {
    const read = readCopySet(name);
    copySets.set(name, read);
    read.then((set) => {
      if (!set?.complete) copySets.delete(name);
    });
  }
  return copySets.get(name);
}

async function readCopySet(name) {
  const r = name === state.set?.name ? {ok: true, body: state.set} : await api(apiSet(name));
  if (!r.ok) return null;
  const all = [];
  const retired = new Map();
  let complete = true;
  const retire = (entries) => {
    for (const e of entries ?? []) {
      if (!retired.has(idPath(e.id))) retired.set(idPath(e.id), e);
    }
  };
  // As the server reads a set, an item's children come before its own
  // retired entries, and a file's items before its.
  const walk = (items) => {
    for (const it of items ?? []) {
      all.push(it);
      walk(it.children);
      retire(it.retired);
    }
  };
  for (const f of r.body.files) {
    if (f.broken) continue;
    const c = f.kind === 'mission' ? {ok: true, body: f} : await api(apiFile(name, f.path));
    complete &&= c.ok;
    walk(c.body?.items);
    retire(c.body?.retired);
  }
  const byPath = new Map();
  for (const it of all) {
    if (!retired.has(idPath(it.id)) && !byPath.has(idPath(it.id))) byPath.set(idPath(it.id), it);
  }
  return {
    complete,
    items: all.filter((it) => byPath.get(idPath(it.id)) === it),
    item(id) {
      const it = byPath.get(idPath(id));
      return it?.id === id ? it : null;
    },
    retiredWith(id) {
      for (let p = idPath(id); ; p = p.slice(0, p.lastIndexOf('.'))) {
        if (retired.has(p)) return retired.get(p);
        if (!p.includes('.')) return null;
      }
    },
  };
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
  if (events || copy) return;
  events = new EventSource(base + 'api/events');
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
