import { SvelteSet } from 'svelte/reactivity';
import { FileService } from '../../bindings/github.com/twinaapp/twina';
import type { Entry } from '../../bindings/github.com/twinaapp/twina/models';

/** App-wide view settings shared by every panel and tab. */
export const settings = $state({ showHidden: false });

export type SortKey = 'name' | 'ext' | 'size' | 'modTime';

export interface Row extends Entry {
  up?: boolean; // the ".." row
}

export class PanelState {
  path = $state('');
  parent = $state('');
  entries = $state<Entry[]>([]);
  cursor = $state(0);
  selected = new SvelteSet<string>();
  sortKey = $state<SortKey>('name');
  sortAsc = $state(true);
  error = $state('');
  loading = $state(false);

  onchange?: () => void;

  rows: Row[] = $derived.by(() => {
    const visible = settings.showHidden ? this.entries : this.entries.filter((e) => !e.hidden);
    const dir = this.sortAsc ? 1 : -1;
    const key = this.sortKey;
    const sorted = [...visible].sort((a, b) => {
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
      let c = 0;
      if (key === 'size') c = a.size - b.size;
      else if (key === 'modTime') c = a.modTime - b.modTime;
      else if (key === 'ext') c = a.ext.localeCompare(b.ext);
      if (c === 0) c = a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' });
      return c * dir;
    });
    if (!this.parent) return sorted;
    const up: Row = {
      name: '..', path: this.parent, isDir: true, isLink: false, hidden: false,
      size: 0, modTime: 0, mode: '', ext: '', up: true,
    };
    return [up, ...sorted];
  });

  current: Row | undefined = $derived(this.rows[this.cursor]);

  /** Selected entries, or the entry under the cursor when nothing is selected. */
  targets(): Entry[] {
    if (this.selected.size > 0) return this.rows.filter((r) => this.selected.has(r.path));
    const c = this.current;
    return c && !c.up ? [c] : [];
  }

  selectionInfo = $derived.by(() => {
    let files = 0, dirs = 0, bytes = 0, selBytes = 0, sel = 0;
    for (const r of this.rows) {
      if (r.up) continue;
      if (r.isDir) dirs++; else { files++; bytes += r.size; }
      if (this.selected.has(r.path)) { sel++; if (!r.isDir) selBytes += r.size; }
    }
    return { files, dirs, bytes, sel, selBytes };
  });

  /** Load a directory. `focus` is the name to place the cursor on afterwards. */
  async load(path: string, focus?: string) {
    this.loading = true;
    try {
      const listing = await FileService.List(path);
      if (!listing) return;
      const changedDir = listing.path !== this.path;
      this.path = listing.path;
      this.parent = listing.parent;
      this.entries = listing.entries ?? [];
      this.error = '';
      if (changedDir) this.selected.clear();
      else for (const p of [...this.selected]) if (!this.entries.some((e) => e.path === p)) this.selected.delete(p);
      const idx = focus ? this.rows.findIndex((r) => r.name === focus) : -1;
      this.cursor = idx >= 0 ? idx : changedDir ? 0 : Math.min(this.cursor, this.rows.length - 1);
      this.onchange?.();
    } catch (err) {
      this.error = errorText(err);
    } finally {
      this.loading = false;
    }
  }

  /** Folder name used for the tab label. */
  title = $derived(this.path.split(/[\\/]/).filter(Boolean).pop() ?? '/');

  refresh() {
    return this.load(this.path, this.current?.name);
  }

  goUp() {
    if (!this.parent) return;
    const name = this.path.split(/[\\/]/).filter(Boolean).pop();
    return this.load(this.parent, name);
  }

  move(delta: number) {
    this.cursor = Math.max(0, Math.min(this.rows.length - 1, this.cursor + delta));
  }

  toggleSelect(row: Row | undefined = this.current) {
    if (!row || row.up) return;
    if (this.selected.has(row.path)) this.selected.delete(row.path);
    else this.selected.add(row.path);
  }

  selectRange(to: number) {
    const [a, b] = [Math.min(this.cursor, to), Math.max(this.cursor, to)];
    for (let i = a; i <= b; i++) {
      const r = this.rows[i];
      if (!r.up) this.selected.add(r.path);
    }
  }

  selectAll() {
    const all = this.rows.filter((r) => !r.up);
    if (all.every((r) => this.selected.has(r.path))) this.selected.clear();
    else for (const r of all) this.selected.add(r.path);
  }

  setSort(key: SortKey) {
    const name = this.current?.name;
    if (this.sortKey === key) this.sortAsc = !this.sortAsc;
    else { this.sortKey = key; this.sortAsc = key === 'name' || key === 'ext'; }
    const idx = this.rows.findIndex((r) => r.name === name);
    if (idx >= 0) this.cursor = idx;
  }

  /** Jump to the first row whose name starts with `prefix`. */
  jumpTo(prefix: string): boolean {
    const p = prefix.toLowerCase();
    const idx = this.rows.findIndex((r) => !r.up && r.name.toLowerCase().startsWith(p));
    if (idx >= 0) this.cursor = idx;
    return idx >= 0;
  }
}

/** One side of the window: a set of tabs, each an independent panel. */
export class Side {
  tabs = $state<PanelState[]>([]);
  index = $state(0);

  constructor(readonly id: 'left' | 'right') {}

  panel: PanelState = $derived(this.tabs[this.index]);

  /** Restore saved tabs, falling back to `home` for missing folders. */
  async restore(home: string) {
    let saved: { paths: string[]; index: number } | null = null;
    try { saved = JSON.parse(localStorage.getItem(`twina.${this.id}.tabs`) ?? 'null'); } catch {}
    const paths = saved?.paths?.length ? saved.paths : [home];
    this.tabs = paths.map(() => this.make());
    this.index = Math.min(saved?.index ?? 0, this.tabs.length - 1);
    await Promise.all(this.tabs.map(async (t, i) => {
      await t.load(paths[i]);
      if (t.error) await t.load(home);
    }));
  }

  private make() {
    const p = new PanelState();
    p.onchange = () => this.save();
    return p;
  }

  save() {
    try {
      localStorage.setItem(`twina.${this.id}.tabs`, JSON.stringify({
        paths: this.tabs.map((t) => t.path), index: this.index,
      }));
    } catch {}
  }

  async open(path = this.panel.path) {
    const tab = this.make();
    tab.sortKey = this.panel.sortKey;
    tab.sortAsc = this.panel.sortAsc;
    this.tabs.splice(this.index + 1, 0, tab);
    this.index++;
    await tab.load(path);
    this.save();
  }

  close(i = this.index) {
    if (this.tabs.length === 1) return;
    this.tabs.splice(i, 1);
    if (this.index > i || this.index === this.tabs.length) this.index--;
    this.save();
  }

  select(i: number) {
    this.index = (i + this.tabs.length) % this.tabs.length;
    this.save();
  }

  refreshAll() {
    return Promise.all(this.tabs.map((t) => t.refresh()));
  }
}

export function errorText(err: unknown): string {
  if (err instanceof Error) return err.message;
  if (typeof err === 'object' && err && 'message' in err) return String((err as { message: unknown }).message);
  return String(err);
}

export function formatSize(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let v = n / 1024, i = 0;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return `${v < 10 ? v.toFixed(1) : Math.round(v)} ${units[i]}`;
}

const dateFmt = new Intl.DateTimeFormat(undefined, {
  year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
});

export function formatDate(ms: number): string {
  return ms ? dateFmt.format(ms) : '';
}
