<script lang="ts">
  import { onMount } from 'svelte';
  import type { FileText } from '../../bindings/github.com/twinaapp/twina/models';
  import { formatSize } from './state.svelte';

  let { file, onclose }: { file: FileText; onclose: () => void } = $props();
  let bodyEl: HTMLElement;

  const name = $derived(file.path.split(/[\\/]/).pop());

  onMount(() => bodyEl.focus());

  function keydown(e: KeyboardEvent) {
    if (e.key === 'Escape' || e.key === 'F3' || e.key === 'q') {
      e.preventDefault();
      e.stopPropagation();
      onclose();
    }
  }
</script>

<div class="viewer" role="dialog" aria-label="View {name}" tabindex="-1" onkeydown={keydown}>
  <header>
    <span class="title">{name}</span>
    <span class="meta">{formatSize(file.size)}{file.truncated ? ' · showing first 2 MB' : ''}</span>
    <button class="btn" onclick={onclose}>Close <kbd>Esc</kbd></button>
  </header>
  <pre bind:this={bodyEl} tabindex="-1">{#if file.binary}Binary file — press F4 or Enter in the panel to open it with the default app.{:else}{file.content}{/if}</pre>
</div>

<style>
  .viewer {
    position: fixed;
    inset: 0;
    display: flex;
    flex-direction: column;
    background: var(--bg);
    z-index: 5;
    padding-top: var(--titlebar);
  }
  header {
    display: flex;
    align-items: center;
    gap: 12px;
    height: 36px;
    padding: 0 12px;
    border-bottom: 1px solid var(--border);
  }
  .title { font-weight: 600; font-size: 13px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .meta { color: var(--muted); font-size: 12px; flex: 1; }
  pre {
    flex: 1;
    margin: 0;
    padding: 12px 16px;
    overflow: auto;
    font-family: var(--mono);
    font-size: 12px;
    line-height: 1.5;
    tab-size: 4;
    outline: none;
    white-space: pre;
  }
</style>
