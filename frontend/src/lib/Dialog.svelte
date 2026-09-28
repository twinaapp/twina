<script lang="ts" module>
  export interface DialogRequest {
    title: string;
    message?: string;
    /** When set, the dialog shows a text input pre-filled with this value. */
    input?: string;
    confirm?: string;
    danger?: boolean;
    /** Extra buttons shown before the confirm button; each resolves with its value. */
    choices?: { label: string; value: string }[];
    resolve: (value: string | null) => void;
  }
</script>

<script lang="ts">
  import { onMount } from 'svelte';

  let { req }: { req: DialogRequest } = $props();
  // svelte-ignore state_referenced_locally
  let value = $state(req.input ?? '');
  let inputEl: HTMLInputElement | undefined = $state();
  let okEl: HTMLButtonElement;
  let actionsEl: HTMLElement;

  onMount(() => {
    if (inputEl) {
      inputEl.focus();
      // Select the base name so typing replaces it but keeps the extension.
      const dot = value.lastIndexOf('.');
      inputEl.setSelectionRange(0, dot > 0 && !value.includes('/') ? dot : value.length);
    } else {
      // Prefer a non-destructive choice (e.g. Skip) as the Enter default.
      (actionsEl.querySelector<HTMLElement>('[data-choice]') ?? okEl).focus();
    }
  });

  function submit(e: SubmitEvent) {
    e.preventDefault();
    req.resolve(req.input !== undefined ? value : '');
  }
</script>

<div class="backdrop" role="presentation" onmousedown={(e) => e.target === e.currentTarget && req.resolve(null)}>
  <!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
  <form
    class="dialog"
    onsubmit={submit}
    onkeydown={(e) => { if (e.key === 'Escape') { e.stopPropagation(); req.resolve(null); } }}
  >
    <h2>{req.title}</h2>
    {#if req.message}<p>{req.message}</p>{/if}
    {#if req.input !== undefined}
      <input bind:this={inputEl} bind:value spellcheck="false" autocomplete="off" />
    {/if}
    <div class="actions" bind:this={actionsEl}>
      <button type="button" class="btn" onclick={() => req.resolve(null)}>Cancel</button>
      {#each req.choices ?? [] as c (c.value)}
        <button type="button" class="btn" data-choice onclick={() => req.resolve(c.value)}>{c.label}</button>
      {/each}
      <button type="submit" class="btn primary" class:danger={req.danger} bind:this={okEl}>{req.confirm ?? 'OK'}</button>
    </div>
  </form>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    display: grid;
    place-items: start center;
    padding-top: 18vh;
    background: var(--scrim);
    z-index: 10;
  }
  .dialog {
    width: min(480px, calc(100vw - 32px));
    padding: 16px;
    border-radius: 10px;
    background: var(--panel);
    border: 1px solid var(--border);
    box-shadow: 0 16px 48px rgb(0 0 0 / 0.35);
  }
  h2 { margin: 0 0 8px; font-size: 14px; font-weight: 600; }
  p { margin: 0 0 12px; color: var(--muted); font-size: 13px; line-height: 1.45; word-break: break-word; white-space: pre-line; }
  input {
    width: 100%;
    margin-bottom: 14px;
    padding: 6px 8px;
    font: inherit;
    font-family: var(--mono);
    font-size: 12px;
    color: var(--text);
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 5px;
    outline: none;
  }
  input:focus { border-color: var(--accent); }
  .actions { display: flex; justify-content: flex-end; gap: 8px; }
</style>
