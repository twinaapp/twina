<script lang="ts">
  import { tick } from 'svelte';
  import { formatDate, formatSize, type Row, type Side, type SortKey } from './state.svelte';
  import Icon, { kindOf } from './Icon.svelte';

  interface Props {
    side: Side;
    active: boolean;
    onactivate: () => void;
    onopen: (row: Row) => void;
  }
  let { side, active, onactivate, onopen }: Props = $props();
  const panel = $derived(side.panel);

  let listEl: HTMLElement;
  let crumbsEl: HTMLElement | undefined = $state();
  let pathInput: HTMLInputElement | undefined = $state();
  let editingPath = $state(false);
  let pathDraft = $state('');

  const columns: { key: SortKey; label: string }[] = [
    { key: 'name', label: 'Name' },
    { key: 'ext', label: 'Ext' },
    { key: 'size', label: 'Size' },
    { key: 'modTime', label: 'Modified' },
  ];

  // Keep the cursor row visible as it moves.
  $effect(() => {
    const i = panel.cursor;
    listEl?.querySelector<HTMLElement>(`[data-i="${i}"]`)?.scrollIntoView({ block: 'nearest' });
  });

  // Keep the deepest part of a long path visible.
  $effect(() => {
    void panel.path;
    if (crumbsEl) crumbsEl.scrollLeft = crumbsEl.scrollWidth;
  });

  export async function editPath() {
    pathDraft = panel.path;
    editingPath = true;
    await tick();
    pathInput?.select();
  }

  function submitPath(e: SubmitEvent) {
    e.preventDefault();
    editingPath = false;
    panel.load(pathDraft.trim());
  }

  function click(e: MouseEvent, i: number, row: Row) {
    onactivate();
    if (e.metaKey || e.ctrlKey) panel.toggleSelect(row);
    else if (e.shiftKey) panel.selectRange(i);
    panel.cursor = i;
  }

  function nameOf(row: Row) {
    if (row.isDir || !row.ext) return row.name;
    return row.name.slice(0, -(row.ext.length + 1));
  }

  const crumbs = $derived.by(() => {
    const sep = panel.path.includes('\\') ? '\\' : '/';
    const parts = panel.path.split(/[\\/]/).filter(Boolean);
    const root = panel.path.startsWith('/') ? '/' : '';
    return parts.map((name, i) => ({ name, path: root + parts.slice(0, i + 1).join(sep) }));
  });

  const info = $derived(panel.selectionInfo);
</script>

<section class="panel" class:active onmousedown={onactivate} role="presentation">
  <div class="tabs" role="tablist">
    {#each side.tabs as tab, i (tab)}
      <div
        class="tab"
        class:current={i === side.index}
        role="tab"
        tabindex="-1"
        aria-selected={i === side.index}
        title={tab.path}
        onmousedown={(e) => { if (e.button === 1) { e.preventDefault(); side.close(i); } else side.select(i); }}
      >
        <span class="tab-label">{tab.title}</span>
        {#if side.tabs.length > 1}
          <button class="tab-close" title="Close tab (⌘W)" onmousedown={(e) => { e.stopPropagation(); side.close(i); }}>×</button>
        {/if}
      </div>
    {/each}
    <button class="tab-add" title="New tab (⌘T)" onclick={() => side.open()}>+</button>
  </div>

  <header class="pathbar">
    {#if editingPath}
      <form onsubmit={submitPath}>
        <input
          bind:this={pathInput}
          bind:value={pathDraft}
          onblur={() => (editingPath = false)}
          onkeydown={(e) => e.key === 'Escape' && (editingPath = false)}
          spellcheck="false"
          autocomplete="off"
        />
      </form>
    {:else}
      <div class="crumbs" bind:this={crumbsEl} ondblclick={editPath} role="navigation">
        <button class="crumb root" onclick={() => panel.load('/')} title="/">/</button>
        {#each crumbs as c, i (c.path)}
          {#if i > 0}<span class="sep">/</span>{/if}
          <button class="crumb" onclick={() => panel.load(c.path)}>{c.name}</button>
        {/each}
      </div>
      <button class="tool" title="Home" onclick={() => panel.load('~')}>~</button>
      <button class="tool" title="Edit path (⌘L)" onclick={editPath}>⋯</button>
    {/if}
  </header>

  <div class="cols">
    {#each columns as col (col.key)}
      <button class="col {col.key}" class:sorted={panel.sortKey === col.key} onclick={() => panel.setSort(col.key)}>
        {col.label}
        {#if panel.sortKey === col.key}<span class="arrow">{panel.sortAsc ? '▲' : '▼'}</span>{/if}
      </button>
    {/each}
  </div>

  <div class="list" bind:this={listEl} role="listbox" aria-label="{side.id} panel">
    {#if panel.error}
      <div class="error">{panel.error}</div>
    {/if}
    {#each panel.rows as row, i (row.path + (row.up ? '..' : ''))}
      <div
        class="row"
        class:cursor={i === panel.cursor}
        class:selected={panel.selected.has(row.path)}
        class:dir={row.isDir}
        class:hidden={row.hidden}
        data-i={i}
        role="option"
        aria-selected={panel.selected.has(row.path)}
        tabindex="-1"
        onmousedown={(e) => click(e, i, row)}
        ondblclick={() => onopen(row)}
      >
        <span class="name">
          <Icon kind={kindOf(row)} link={row.isLink} />
          <span class="label">{row.up ? '..' : nameOf(row)}</span>
        </span>
        <span class="ext">{row.isDir ? '' : row.ext}</span>
        <span class="size">{row.up ? '' : row.isDir ? '<DIR>' : formatSize(row.size)}</span>
        <span class="modTime">{formatDate(row.modTime)}</span>
      </div>
    {/each}
  </div>

  <footer class="status">
    {#if info.sel > 0}
      <span class="sel">{info.sel} selected · {formatSize(info.selBytes)}</span>
    {:else}
      <span>{panel.current && !panel.current.up ? panel.current.name : ''}</span>
    {/if}
    <span class="totals">{info.dirs} dirs · {info.files} files · {formatSize(info.bytes)}</span>
  </footer>
</section>

<style>
  .panel {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
    background: var(--panel);
    border: 1px solid var(--border);
    border-radius: 8px;
    overflow: hidden;
  }
  .panel.active { border-color: var(--accent-dim); }

  .pathbar {
    display: flex;
    align-items: center;
    gap: 4px;
    height: 32px;
    padding: 0 6px 0 8px;
    border-bottom: 1px solid var(--border);
    background: var(--header);
  }
  .panel.active .pathbar { background: var(--header-active); }
  .pathbar form { flex: 1; }
  .pathbar input {
    width: 100%;
    font: inherit;
    font-family: var(--mono);
    font-size: 12px;
    padding: 3px 6px;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--accent);
    border-radius: 4px;
    outline: none;
  }
  .crumbs {
    flex: 1;
    display: flex;
    align-items: center;
    min-width: 0;
    white-space: nowrap;
    font-family: var(--mono);
    font-size: 12px;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .crumbs::-webkit-scrollbar { display: none; }
  .crumb { flex: none; }
  .crumb {
    padding: 2px 3px;
    border-radius: 3px;
    color: var(--muted);
  }
  .crumb:last-child { color: var(--text); font-weight: 600; }
  .crumb:hover { background: var(--hover); color: var(--text); }
  .sep { color: var(--faint); }
  .tool {
    width: 24px;
    height: 22px;
    border-radius: 4px;
    color: var(--muted);
    font-family: var(--mono);
  }
  .tool:hover { background: var(--hover); color: var(--text); }

  .tabs {
    display: flex;
    align-items: stretch;
    height: 28px;
    background: var(--bg);
    border-bottom: 1px solid var(--border);
    overflow-x: auto;
    scrollbar-width: none;
  }
  .tabs::-webkit-scrollbar { display: none; }
  .tab {
    display: flex;
    align-items: center;
    gap: 4px;
    min-width: 0;
    max-width: 180px;
    padding: 0 6px 0 12px;
    border-right: 1px solid var(--border);
    font-size: 12px;
    color: var(--muted);
    cursor: default;
    outline: none;
  }
  .tab:hover { color: var(--text); }
  .tab.current { background: var(--header); color: var(--text); font-weight: 600; box-shadow: inset 0 -2px 0 var(--faint); }
  .panel.active .tab.current { background: var(--header-active); box-shadow: inset 0 -2px 0 var(--accent); }
  .tab-label { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .tab-close {
    width: 16px;
    height: 16px;
    border-radius: 3px;
    line-height: 1;
    color: var(--faint);
    visibility: hidden;
  }
  .tab:hover .tab-close, .tab.current .tab-close { visibility: visible; }
  .tab-close:hover { background: var(--hover); color: var(--text); }
  .tab-add { width: 28px; flex: none; color: var(--faint); font-size: 15px; }
  .tab-add:hover { color: var(--text); background: var(--hover); }

  .cols, .row {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 56px 80px 136px;
    column-gap: 10px;
    padding: 0 10px;
  }
  .cols {
    height: 24px;
    align-items: center;
    border-bottom: 1px solid var(--border);
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .col { text-align: left; color: var(--faint); display: flex; gap: 4px; align-items: center; }
  .col:hover, .col.sorted { color: var(--muted); }
  .col.size { justify-content: flex-end; }
  .arrow { font-size: 8px; }

  .list {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    font-size: 13px;
    padding: 2px 0;
  }
  .row {
    height: 22px;
    align-items: center;
    cursor: default;
    color: var(--text);
    white-space: nowrap;
    outline: none;
  }
  .row > span { overflow: hidden; text-overflow: ellipsis; }
  .row.hidden { color: var(--muted); }
  .row.selected { color: var(--sel); }
  .row.selected .label { font-weight: 600; }
  .row.cursor { background: var(--cursor-inactive); }
  .panel.active .row.cursor { background: var(--cursor); color: var(--cursor-text); }
  .panel.active .row.cursor.selected { color: var(--sel-on-cursor); }

  .name { display: flex; align-items: center; gap: 6px; min-width: 0; }
  .row.hidden :global(.icon) { opacity: 0.55; }
  .panel.active .row.cursor :global(.icon) { color: inherit; opacity: 1; }
  .panel.active .row.cursor :global(.badge) { fill: var(--cursor); }
  .label { overflow: hidden; text-overflow: ellipsis; }
  .ext { color: var(--muted); }
  .row.cursor .ext, .row.cursor .size, .row.cursor .modTime { color: inherit; }
  .size { text-align: right; font-variant-numeric: tabular-nums; color: var(--muted); }
  .modTime { font-variant-numeric: tabular-nums; color: var(--muted); font-size: 12px; }

  .error {
    margin: 8px 10px;
    padding: 6px 8px;
    border-radius: 4px;
    background: var(--danger-bg);
    color: var(--danger);
    font-size: 12px;
  }

  .status {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    height: 24px;
    align-items: center;
    padding: 0 10px;
    border-top: 1px solid var(--border);
    font-size: 11px;
    color: var(--muted);
    white-space: nowrap;
  }
  .status > span:first-child { overflow: hidden; text-overflow: ellipsis; }
  .sel { color: var(--sel); font-weight: 600; }
</style>
