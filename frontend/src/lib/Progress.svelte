<script lang="ts">
  import type { Progress } from '../../bindings/github.com/twinaapp/twina/models';
  import { formatSize } from './state.svelte';

  let { title, progress, oncancel }: { title: string; progress: Progress | null; oncancel: () => void } = $props();

  const pct = $derived(
    progress && progress.phase !== 'scan' && progress.total > 0 ? Math.min(100, (progress.done / progress.total) * 100) : null,
  );
</script>

<div class="backdrop">
  <div class="box" role="dialog" aria-label={title}>
    <h2>{title}</h2>
    <div class="bar" class:indeterminate={pct === null}>
      <div class="fill" style:width={pct === null ? undefined : `${pct}%`}></div>
    </div>
    <div class="meta">
      {#if !progress || progress.phase === 'scan'}
        <span>Preparing…</span>
      {:else}
        <span class="current">{progress.current}</span>
        <span class="nums">
          {progress.files} / {progress.totalFiles} files · {formatSize(progress.done)} of {formatSize(progress.total)}
        </span>
      {/if}
    </div>
    <div class="actions">
      <button class="btn" onclick={oncancel}>Cancel <kbd>Esc</kbd></button>
    </div>
  </div>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: start center;
    padding-top: 18vh;
    background: var(--scrim);
    z-index: 9;
  }
  .box {
    width: min(460px, calc(100vw - 32px));
    padding: 16px;
    border-radius: 10px;
    background: var(--panel);
    border: 1px solid var(--border);
    box-shadow: 0 16px 48px rgb(0 0 0 / 0.35);
  }
  h2 { margin: 0 0 12px; font-size: 14px; font-weight: 600; }
  .bar { height: 6px; border-radius: 3px; background: var(--cursor-inactive); overflow: hidden; }
  .fill { height: 100%; background: var(--accent); transition: width 0.12s linear; }
  .indeterminate .fill { width: 30%; animation: slide 1.1s ease-in-out infinite; }
  @keyframes slide { from { transform: translateX(-100%); } to { transform: translateX(340%); } }
  .meta {
    display: flex;
    justify-content: space-between;
    gap: 12px;
    margin: 8px 0 14px;
    font-size: 12px;
    color: var(--muted);
    white-space: nowrap;
  }
  .current { overflow: hidden; text-overflow: ellipsis; min-width: 0; }
  .nums { font-variant-numeric: tabular-nums; }
  .actions { display: flex; justify-content: flex-end; }
</style>
