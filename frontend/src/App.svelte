<script lang="ts">
  import { onMount } from 'svelte';
  import { CancelError, Events, type CancellablePromise } from '@wailsio/runtime';
  import { FileService } from '../bindings/github.com/twinaapp/twina';
  import type { FileText, Progress as ProgressData } from '../bindings/github.com/twinaapp/twina/models';
  import Panel from './lib/Panel.svelte';
  import Dialog, { type DialogRequest } from './lib/Dialog.svelte';
  import Viewer from './lib/Viewer.svelte';
  import Progress from './lib/Progress.svelte';
  import { Side, errorText, settings, type Row } from './lib/state.svelte';

  const left = new Side('left');
  const right = new Side('right');
  let activeId = $state<'left' | 'right'>('left');
  const activeSide = $derived(activeId === 'left' ? left : right);
  const otherSide = $derived(activeId === 'left' ? right : left);
  const active = $derived(activeSide.panel);
  const other = $derived(otherSide.panel);
  const ready = $derived(!!left.panel && !!right.panel);

  let panelRefs: Record<'left' | 'right', ReturnType<typeof Panel> | undefined> = $state({ left: undefined, right: undefined });
  let dialog = $state<DialogRequest | null>(null);
  let viewing = $state<FileText | null>(null);

  // The running copy/move/delete, if any. The progress box only appears when
  // an operation takes long enough to notice.
  let job = $state<{ id: string; title: string; visible: boolean; progress: ProgressData | null; cancel: () => void } | null>(null);
  let jobDone: ((p: ProgressData) => void) | null = null;

  let search = '';
  let searchTimer: ReturnType<typeof setTimeout>;

  onMount(() => {
    try { settings.showHidden = localStorage.getItem('twina.hidden') === '1'; } catch {}
    FileService.Home().then((home) => Promise.all([left.restore(home), right.restore(home)]));

    return Events.On('fileop:progress', (ev) => {
      const p = ev.data;
      if (!job || p.job !== job.id) return;
      job.progress = p;
      if (p.phase === 'done') jobDone?.(p);
    });
  });

  $effect(() => {
    const v = settings.showHidden ? '1' : '0';
    try { localStorage.setItem('twina.hidden', v); } catch {}
  });

  function ask(req: Omit<DialogRequest, 'resolve'>): Promise<string | null> {
    return new Promise((resolve) => {
      dialog = { ...req, resolve: (v) => { dialog = null; resolve(v); } };
    });
  }

  async function alertError(title: string, err: unknown) {
    await ask({ title, message: errorText(err), confirm: 'OK' });
  }

  function refreshAll() {
    return Promise.all([left.refreshAll(), right.refreshAll()]);
  }

  /** Run a long file operation with a delayed progress box and Cancel. */
  async function runJob(title: string, start: (id: string) => CancellablePromise<void>, reportsProgress = true) {
    const id = crypto.randomUUID();
    const finished = new Promise<unknown>((resolve) => (jobDone = resolve));
    const call = start(id);
    let cancelled = false;
    job = {
      id, title, visible: false, progress: null,
      cancel: () => { cancelled = true; call.cancel(); },
    };
    const showTimer = setTimeout(() => job && (job.visible = true), 250);
    try {
      await call;
    } catch (err) {
      if (!(err instanceof CancelError) && !cancelled) {
        clearTimeout(showTimer);
        job = null;
        await alertError(`${title} failed`, err);
      }
    }
    if (cancelled && reportsProgress) {
      // Wait for the backend to finish removing partial copies before refreshing.
      await Promise.race([finished, new Promise((r) => setTimeout(r, 5000))]);
    }
    clearTimeout(showTimer);
    job = null;
    jobDone = null;
    await refreshAll();
  }

  function describe(targets: { name: string }[]) {
    return targets.length === 1 ? `“${targets[0].name}”` : `${targets.length} items`;
  }

  const baseName = (p: string) => p.split(/[\\/]/).filter(Boolean).pop() ?? p;

  async function open(row: Row | undefined = active.current) {
    if (!row) return;
    if (row.up) return active.goUp();
    if (row.isDir) return active.load(row.path);
    try { await FileService.Open(row.path); } catch (err) { alertError('Open failed', err); }
  }

  async function view() {
    const row = active.current;
    if (!row || row.up) return;
    if (row.isDir) return active.load(row.path);
    try {
      viewing = await FileService.ReadText(row.path);
    } catch (err) {
      alertError('Cannot view file', err);
    }
  }

  async function transfer(kind: 'Copy' | 'Move') {
    const targets = active.targets();
    if (!targets.length) return;
    const dest = await ask({
      title: kind,
      message: `${kind} ${describe(targets)} to:`,
      input: other.path,
      confirm: kind,
    });
    if (dest === null) return;

    let sources = targets.map((t) => t.path);
    let overwrite = false;
    let conflicts: string[];
    try {
      conflicts = (await FileService.Conflicts(sources, dest)) ?? [];
    } catch (err) {
      return alertError(`${kind} failed`, err);
    }
    if (conflicts.length) {
      const shown = conflicts.slice(0, 6).map((n) => `  • ${n}`).join('\n');
      const more = conflicts.length > 6 ? `\n  …and ${conflicts.length - 6} more` : '';
      const choice = await ask({
        title: conflicts.length === 1 ? 'An item with this name already exists' : `${conflicts.length} items already exist`,
        message: `${shown}${more}\n\nReplace replaces files and merges folders.`,
        choices: [{ label: 'Skip', value: 'skip' }],
        confirm: 'Replace',
        danger: true,
      });
      if (choice === null) return;
      if (choice === 'skip') {
        sources = sources.filter((p) => !conflicts.includes(baseName(p)));
        if (!sources.length) return;
      } else {
        overwrite = true;
      }
    }

    active.selected.clear();
    await runJob(kind === 'Copy' ? 'Copying' : 'Moving', (id) =>
      kind === 'Copy' ? FileService.Copy(id, sources, dest, overwrite) : FileService.Move(id, sources, dest, overwrite),
    );
  }

  async function rename() {
    const row = active.current;
    if (!row || row.up) return;
    const name = await ask({ title: 'Rename', input: row.name, confirm: 'Rename' });
    if (!name || name === row.name) return;
    try {
      await FileService.Rename(row.path, name);
      await active.load(active.path, name);
    } catch (err) {
      await alertError('Rename failed', err);
    }
  }

  async function mkdir() {
    const name = await ask({ title: 'New folder', input: '', confirm: 'Create' });
    if (!name) return;
    try {
      await FileService.MakeDir(active.path, name);
      await active.load(active.path, name);
      if (other.path === active.path) await other.refresh();
    } catch (err) {
      await alertError('Create folder failed', err);
    }
  }

  async function remove(permanent = false) {
    const targets = active.targets();
    if (!targets.length) return;
    const paths = targets.map((t) => t.path);

    if (permanent) {
      const ok = await ask({
        title: 'Delete permanently',
        message: `Permanently delete ${describe(targets)}? This can't be undone.`,
        confirm: 'Delete',
        danger: true,
      });
      if (ok === null) return;
      active.selected.clear();
      return runJob('Deleting', () => FileService.DeletePermanently(paths), false);
    }

    const ok = await ask({ title: 'Move to Trash', message: `Move ${describe(targets)} to the Trash?`, confirm: 'Move to Trash' });
    if (ok === null) return;
    active.selected.clear();
    let failed: string[] = [];
    try {
      const res = await FileService.Trash(paths);
      failed = res?.failed ?? [];
      if (failed.length) {
        const again = await ask({
          title: "Couldn't move to Trash",
          message: `${describe(failed.map((p) => ({ name: baseName(p) })))} couldn't be moved to the Trash:\n${res?.message}\n\nDelete permanently instead? This can't be undone.`,
          confirm: 'Delete permanently',
          danger: true,
        });
        if (again !== null) return runJob('Deleting', () => FileService.DeletePermanently(failed), false);
      }
    } catch (err) {
      await alertError('Move to Trash failed', err);
    }
    await refreshAll();
  }

  function swap() {
    const [l, r] = [left.panel.path, right.panel.path];
    left.panel.load(r);
    right.panel.load(l);
  }

  function pageSize() {
    const list = document.querySelector('.panel .list');
    return Math.max(1, Math.floor((list?.clientHeight ?? 440) / 22) - 1);
  }

  function keydown(e: KeyboardEvent) {
    if (!ready) return;
    if (job?.visible) {
      if (e.key === 'Escape') { e.preventDefault(); job.cancel(); }
      return;
    }
    if (dialog || viewing || job) return;
    const t = e.target as HTMLElement;
    if (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA') return;

    const mod = e.metaKey || e.ctrlKey;
    const p = active;
    let handled = true;

    switch (e.key) {
      case 'Tab':
        if (e.ctrlKey) activeSide.select(activeSide.index + (e.shiftKey ? -1 : 1));
        else activeId = activeId === 'left' ? 'right' : 'left';
        break;
      case 'ArrowDown': if (e.shiftKey) p.toggleSelect(); p.move(1); break;
      case 'ArrowUp':
        if (mod) p.goUp();
        else { if (e.shiftKey) p.toggleSelect(); p.move(-1); }
        break;
      case 'ArrowLeft': if (mod) left.panel.load(p.path); else handled = false; break;
      case 'ArrowRight': if (mod) right.panel.load(p.path); else handled = false; break;
      case 'PageDown': p.move(pageSize()); break;
      case 'PageUp': p.move(-pageSize()); break;
      case 'Home': p.cursor = 0; break;
      case 'End': p.cursor = p.rows.length - 1; break;
      case 'Enter': open(); break;
      case 'Backspace':
        if (mod) remove(e.altKey); else p.goUp();
        break;
      case 'Insert': p.toggleSelect(); p.move(1); break;
      case 'Delete': case 'F8': remove(e.shiftKey); break;
      case 'F2': rename(); break;
      case 'F3': view(); break;
      case 'F4': if (p.current && !p.current.isDir) FileService.Open(p.current.path); break;
      case 'F5': transfer('Copy'); break;
      case 'F6': if (e.shiftKey) rename(); else transfer('Move'); break;
      case 'F7': mkdir(); break;
      case ' ':
        if (search) { handled = quickSearch(' '); break; }
        p.toggleSelect(); p.move(1);
        break;
      default:
        handled = false;
    }

    if (!handled && mod && !e.altKey) {
      handled = true;
      const k = e.key.toLowerCase();
      if (/^[1-9]$/.test(k)) {
        activeSide.select(k === '9' ? activeSide.tabs.length - 1 : Math.min(+k - 1, activeSide.tabs.length - 1));
      } else switch (k) {
        case 'a': p.selectAll(); break;
        case 'r': refreshAll(); break;
        case 'l': panelRefs[activeId]?.editPath(); break;
        case 'u': swap(); break;
        case 'o': other.load(p.path); break;
        case 'd': p.selected.clear(); break;
        case 't': activeSide.open(); break;
        case 'w': activeSide.close(); break;
        case '[': case '{': if (e.shiftKey) activeSide.select(activeSide.index - 1); else p.goUp(); break;
        case ']': case '}': if (e.shiftKey) activeSide.select(activeSide.index + 1); else handled = false; break;
        case '.': case '>': settings.showHidden = !settings.showHidden; break;
        default: handled = false;
      }
    } else if (!handled && e.key === 'Escape') {
      p.selected.clear();
      handled = true;
    } else if (!handled && !mod && !e.altKey && e.key.length === 1) {
      handled = quickSearch(e.key);
    }

    if (handled) e.preventDefault();
  }

  /** Type-to-jump: consecutive keystrokes build a prefix that moves the cursor. */
  function quickSearch(ch: string) {
    clearTimeout(searchTimer);
    searchTimer = setTimeout(() => (search = ''), 900);
    if (active.jumpTo(search + ch)) search += ch;
    else if (active.jumpTo(ch)) search = ch;
    return true;
  }

  const keys: { key: string; label: string; action: () => void }[] = [
    { key: 'F2', label: 'Rename', action: rename },
    { key: 'F3', label: 'View', action: view },
    { key: 'F4', label: 'Open', action: () => open() },
    { key: 'F5', label: 'Copy', action: () => transfer('Copy') },
    { key: 'F6', label: 'Move', action: () => transfer('Move') },
    { key: 'F7', label: 'MkDir', action: mkdir },
    { key: 'F8', label: 'Delete', action: () => remove() },
  ];
</script>

<svelte:window onkeydown={keydown} />

<div class="app">
  <div class="titlebar">
    <span class="title">Twina</span>
    <label class="toggle" title="Show hidden files (⌘.)">
      <input type="checkbox" bind:checked={settings.showHidden} /> Hidden
    </label>
  </div>

  <main class="panels">
    {#if ready}
      <Panel
        bind:this={panelRefs.left}
        side={left}
        active={activeId === 'left'}
        onactivate={() => (activeId = 'left')}
        onopen={(row) => { activeId = 'left'; open(row); }}
      />
      <Panel
        bind:this={panelRefs.right}
        side={right}
        active={activeId === 'right'}
        onactivate={() => (activeId = 'right')}
        onopen={(row) => { activeId = 'right'; open(row); }}
      />
    {/if}
  </main>

  <nav class="fkeys">
    {#each keys as k (k.key)}
      <button onmousedown={(e) => e.preventDefault()} onclick={k.action}><kbd>{k.key}</kbd>{k.label}</button>
    {/each}
    <span class="spacer"></span>
    <span class="hint">Tab switch · Space select · ⌘T tab · ⌘U swap · ⌘L path</span>
  </nav>
</div>

{#if viewing}
  <Viewer file={viewing} onclose={() => (viewing = null)} />
{/if}

{#if job?.visible}
  <Progress title={job.title} progress={job.progress} oncancel={job.cancel} />
{/if}

{#if dialog}
  <Dialog req={dialog} />
{/if}

<style>
  .app {
    display: flex;
    flex-direction: column;
    height: 100vh;
    padding: 0 8px;
  }
  .titlebar {
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    height: var(--titlebar);
    flex: none;
    --wails-draggable: drag;
    user-select: none;
  }
  .title { font-size: 13px; font-weight: 600; color: var(--muted); }
  .toggle {
    position: absolute;
    right: 4px;
    display: flex;
    align-items: center;
    gap: 5px;
    font-size: 12px;
    color: var(--muted);
    --wails-draggable: no-drag;
  }
  .toggle input { accent-color: var(--accent); margin: 0; }

  .panels {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .fkeys {
    display: flex;
    align-items: center;
    gap: 2px;
    height: 36px;
    flex: none;
    font-size: 12px;
    overflow: hidden;
    white-space: nowrap;
  }
  .fkeys button {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px 4px 4px;
    border-radius: 5px;
    color: var(--text);
  }
  .fkeys button:hover { background: var(--hover); }
  .spacer { flex: 1; }
  .hint { color: var(--faint); font-size: 11px; }
  @media (max-width: 1000px) { .hint { display: none; } }
</style>
