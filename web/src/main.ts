import type { BasketLine, CatalogItem, CompareResponse, Meta, StoreResult } from './types.ts';

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;

/** The Go API can live on another origin (e.g. the UI on GitHub Pages and the
 *  API on Fly.io). Set VITE_API_BASE at build time to point at it. */
const API_BASE = (import.meta.env.VITE_API_BASE ?? '').replace(/\/$/, '');
const api = (path: string) => `${API_BASE}${path}`;

let meta: Meta | null = null;
/** Selected catalogue items and the quantity of each. */
const basket = new Map<string, number>();

async function loadMeta(): Promise<Meta> {
  const res = await fetch(api('/api/meta'));
  if (res.ok) return (await res.json()) as Meta;
  // Static mode (GitHub Pages without an API): fall back to the snapshot.
  const snap = await fetch('./data/meta.json');
  if (snap.ok) return (await snap.json()) as Meta;
  throw new Error(`meta: ${res.status}`);
}

function fmt(n: number): string {
  return '₪' + n.toFixed(2);
}

function renderCatalog(filter = '') {
  const box = $('catalog');
  box.innerHTML = '';
  const f = filter.trim();
  for (const item of meta!.catalog) {
    if (f && !item.he.includes(f) && !item.en.toLowerCase().includes(f.toLowerCase())) continue;
    const on = basket.has(item.id);
    const chip = document.createElement('label');
    chip.className = 'chip' + (on ? ' on' : '');
    chip.innerHTML = `
      <input type="checkbox" ${on ? 'checked' : ''} data-id="${item.id}" />
      <span class="name">${item.he}<br><span class="unit">${item.en} · ${item.unit === 'kg' ? 'לק״ג' : 'ליח׳'}</span></span>
      <input class="qty" type="number" min="0" step="0.5" value="${on ? basket.get(item.id) : item.defaultQty}"
             data-qty="${item.id}" ${on ? '' : 'disabled'} />`;
    box.appendChild(chip);
  }
  $('basketCount').textContent = `${basket.size} מוצרים`;
}

function renderSummary(data: CompareResponse) {
  const el = $('summary');
  const s = data.summary;
  if (!s || data.results.length === 0) {
    el.hidden = true;
    return;
  }
  const maxFound = Math.max(...data.results.map((r) => r.found));
  const cheapest = data.results.filter((r) => r.found === maxFound)[0]!;
  const lfl = s.likeForLike[0];
  el.innerHTML = `
    <div class="card">
      <h3>הסל הזול ביותר</h3>
      <div class="big">${fmt(cheapest!.total)}</div>
      <small>${cheapest!.name} · ${cheapest!.km.toFixed(1)} ק״מ · ${cheapest!.found} מוצרים</small>
    </div>
    <div class="card">
      <h3>השוואה הוגנת (${s.commonCount} מוצרים משותפים)</h3>
      <div class="big">${lfl ? fmt(lfl.total) : '—'}</div>
      <small>${lfl ? lfl.name : 'אין מוצרים משותפים לכל החנויות'}</small>
    </div>
    <div class="card">
      <h3>איסוף מהזול בכל מוצר</h3>
      <div class="big">${fmt(s.cherryPick)}</div>
      <small>${s.cherryMissing ? `${s.cherryMissing} מוצרים לא נמצאו` : 'כל המוצרים נמצאו'}</small>
    </div>`;
  el.hidden = false;
}

function renderResults(data: CompareResponse) {
  const box = $('results');
  box.innerHTML = '';
  if (data.results.length === 0) {
    box.innerHTML = `<div class="status">לא נמצאו חנויות עם מחירים ליד הכתובת. נסו להגדיל את הרדיוס.</div>`;
    return;
  }
  const best = data.results[0]!.total;
  data.results.forEach((r, i) => {
    box.appendChild(storeCard(r, i, best));
  });
}

function catalogById(r: StoreResult): Map<string, CatalogItem> {
  const m = new Map<string, CatalogItem>();
  for (const id of Object.keys(r.lines)) {
    const c = meta!.catalog.find((x) => x.id === id);
    if (c) m.set(id, c);
  }
  return m;
}

function storeCard(r: StoreResult, i: number, best: number): HTMLElement {
  const cat = catalogById(r);
  const diff = r.total - best;
  const lines = [...cat.entries()]
    .map(([id, c]) => {
      const l = r.lines[id]!;
      return `<div class="line"><span class="lname">${c.he}<br><span class="lprod">${l.product}</span></span>
        <span class="lprice">${fmt(l.price)} <span class="lprod">${l.unit}</span></span></div>`;
    })
    .join('');
  const missing = r.missing.length
    ? `<div class="line missing"><span class="lname">חסרים ${r.missing.length} מוצרים</span></div>`
    : '';
  const det = document.createElement('details');
  det.className = 'store' + (i === 0 ? ' best' : '');
  det.innerHTML = `
    <summary>
      <span class="rank">${i + 1}</span>
      <span class="who"><span class="n">${r.name}</span><br><span class="a">${r.address}${r.city ? ', ' + r.city : ''}</span></span>
      <span class="badges">
        <span class="badge">${r.km.toFixed(1)} ק״מ</span>
        <span class="badge ${r.approx ? 'warn' : 'ok'}">${r.approx ? 'מיקום מקורב' : 'מדויק'}</span>
        <span class="badge ${r.missing.length === 0 ? 'ok' : 'warn'}">${r.found}/${r.found + r.missing.length} מוצרים</span>
        ${i > 0 && diff > 0 ? `<span class="badge">+${fmt(diff)}</span>` : ''}
      </span>
      <span class="price">${fmt(r.total)}</span>
    </summary>
    <div class="lines">${lines}${missing}</div>`;
  return det;
}

function setStatus(msg: string | null, isError = false) {
  const el = $('status');
  if (!msg) {
    el.hidden = true;
    return;
  }
  el.hidden = false;
  el.className = 'status' + (isError ? ' err' : '');
  el.textContent = msg;
}

async function compare() {
  if (basket.size === 0) {
    setStatus('בחרו לפחות מוצר אחד מהסל.', true);
    return;
  }
  const btn = $<HTMLButtonElement>('go');
  btn.disabled = true;
  $('results').innerHTML = '';
  $('summary').hidden = true;
  setStatus('מאתר חנויות, מוריד מחירים ומחשב… (בפעם הראשונה לכתובת זו זה עשוי לקחת עד דקה)');

  const items: BasketLine[] = [...basket.entries()].map(([itemId, qty]) => ({ itemId, qty }));
  const body = {
    address: $<HTMLInputElement>('address').value,
    radiusKm: Number($<HTMLInputElement>('radius').value) || 4,
    items,
  };
  try {
    const res = await fetch(api('/api/compare'), {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
    const data = (await res.json()) as CompareResponse;
    if (data.error) {
      setStatus(data.error, true);
      return;
    }
    setStatus(
      data.skipped && data.skipped.length
        ? `הוצגו ${data.results.length} חנויות. ${data.skipped.length} חנויות דולגו (אין קובץ מחירים היום).`
        : null,
    );
    renderSummary(data);
    renderResults(data);
  } catch (e) {
    setStatus(
      API_BASE === '' && location.protocol.startsWith('http') && !location.host.includes('localhost')
        ? 'הממשק הסטטי אינו כולל מחירים חיים. חברו אותו ל-API (VITE_API_BASE) או הריצו את השרת.'
        : 'שגיאה בשליפת הנתונים: ' + (e as Error).message,
      true,
    );
  } finally {
    btn.disabled = false;
  }
}

function wire() {
  $('catalog').addEventListener('change', (ev) => {
    const t = ev.target as HTMLInputElement;
    if (t.dataset.id) {
      const id = t.dataset.id;
      if (t.checked) basket.set(id, meta!.catalog.find((c) => c.id === id)!.defaultQty);
      else basket.delete(id);
      renderCatalog($<HTMLInputElement>('filter').value);
    } else if (t.dataset.qty) {
      basket.set(t.dataset.qty, Math.max(0, Number(t.value) || 0));
      $('basketCount').textContent = `${basket.size} מוצרים`;
    }
  });
  $('filter').addEventListener('input', (e) => renderCatalog((e.target as HTMLInputElement).value));
  $('clear').addEventListener('click', () => {
    basket.clear();
    renderCatalog($<HTMLInputElement>('filter').value);
  });
  $('go').addEventListener('click', compare);
  $('address').addEventListener('keydown', (e) => {
    if ((e as KeyboardEvent).key === 'Enter') compare();
  });
}

async function main() {
  try {
    meta = await loadMeta();
  } catch (e) {
    setStatus('לא ניתן לטעון את הקטלוג: ' + (e as Error).message, true);
    return;
  }
  // A sensible default basket so the page is useful on first load.
  for (const id of ['milk', 'eggs', 'bread', 'cucumber', 'tomato', 'chicken-breast', 'yellow-cheese']) {
    const item = meta.catalog.find((c) => c.id === id);
    if (item) basket.set(id, item.defaultQty);
  }
  renderCatalog();
  wire();
}

void main();
