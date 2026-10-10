<script lang="ts">
  import { onMount } from 'svelte';
  import type { FileText } from '../../bindings/github.com/twinaapp/twina/models';
  import { formatSize } from './state.svelte';

  let { file, onclose, onstep }: { file: FileText; onclose: () => void; onstep: (delta: number) => void } = $props();
  let bodyEl = $state<HTMLElement>();

  const name = $derived(file.path.split(/[\\/]/).pop());

  // Images: fit to the window, or actual size (Space or click to toggle).
  // Videos always fit, and play in the webview's own player.
  // Sizes are worked out here rather than left to CSS so fit never overflows.
  const media = $derived(file.image || file.video);
  let fit = $state(true);
  let broken = $state(false);
  let natural = $state<{ w: number; h: number } | null>(null);
  let areaW = $state(0);
  let areaH = $state(0);
  const src = $derived(`/localfile?path=${encodeURIComponent(file.path)}`);

  const shown = $derived.by(() => {
    if (!natural || !areaW || !areaH) return null;
    // An SVG without a width or height reports 0: give it the whole area.
    if (!natural.w || !natural.h) return { w: areaW, h: areaH, zoom: 0 };
    const zoom = fit || file.video ? Math.min(1, areaW / natural.w, areaH / natural.h) : 1;
    return { w: natural.w * zoom, h: natural.h * zoom, zoom };
  });

  // The image on screen. The next one loads and decodes off-screen and only
  // then replaces it, so stepping through a folder doesn't flash an empty frame.
  let img = $state<HTMLImageElement | null>(null);

  // Reruns for every new file (it reads src).
  $effect(() => {
    const url = src;
    if (!file.image) {
      img = natural = null;
      broken = false;
      return;
    }
    const next = new Image();
    let stale = false;
    next.alt = name ?? '';
    next.onload = () => next.decode().catch(() => {}).then(() => {
      if (stale) return;
      img = next;
      natural = { w: next.naturalWidth, h: next.naturalHeight };
      broken = false;
    });
    next.onerror = () => {
      if (stale) return;
      img = natural = null;
      broken = true;
    };
    next.src = url;
    return () => { stale = true; };
  });

  $effect(() => {
    if (!img) return;
    img.style.width = shown ? `${shown.w}px` : '';
    img.style.height = shown ? `${shown.h}px` : '';
    img.style.visibility = shown ? '' : 'hidden';
  });

  let video = $state<HTMLVideoElement>();

  function videoReady() {
    if (video) natural = { w: video.videoWidth, h: video.videoHeight };
  }

  onMount(() => bodyEl?.focus());

  // Runs in the capture phase so a focused video's own controls don't also act
  // on Space and the arrows.
  function keydown(e: KeyboardEvent) {
    const left = e.key === 'ArrowLeft', right = e.key === 'ArrowRight';
    if (e.key === 'Escape' || e.key === 'F3' || e.key === 'q') {
      onclose();
    } else if (media && (left || right) && e.shiftKey && video) {
      video.currentTime += left ? -5 : 5;
    } else if (media && (left || right)) {
      onstep(left ? -1 : 1);
    } else if (e.key === ' ' && file.video && video && !broken) {
      if (video.paused) video.play(); else video.pause();
    } else if (e.key === ' ' && file.image) {
      fit = !fit;
    } else {
      return;
    }
    e.preventDefault();
    e.stopPropagation();
  }
</script>

<div class="viewer" role="dialog" aria-label="View {name}" tabindex="-1" onkeydowncapture={keydown}>
  <header>
    <span class="title">{name}</span>
    <span class="meta">{formatSize(file.size)}{natural?.w ? ` · ${natural.w} × ${natural.h}` : ''}{shown?.zoom ? ` · ${Math.round(shown.zoom * 100)}%` : ''}{file.truncated ? ' · showing first 2 MB' : ''}</span>
    {#if media}
      <button class="btn" onclick={() => onstep(-1)} title="Previous file">←</button>
      <button class="btn" onclick={() => onstep(1)} title="Next file">→</button>
    {/if}
    {#if file.image && !broken}
      <button class="btn" onclick={() => (fit = !fit)}>{fit ? 'Actual size' : 'Fit'} <kbd>Space</kbd></button>
    {/if}
    <button class="btn" onclick={onclose}>Close <kbd>Esc</kbd></button>
  </header>
  {#if media}
    <!-- svelte-ignore a11y_click_events_have_key_events (Space toggles from the dialog) -->
    <div class="image" class:fit class:video={file.video} bind:this={bodyEl} bind:clientWidth={areaW} bind:clientHeight={areaH}
      tabindex="-1" role="presentation" onclick={() => { if (file.image) fit = !fit; }}>
      {#if broken}
        <p class="note">This {file.video ? 'video' : 'image'} format can't be {file.video ? 'played' : 'shown'} here — press F4 or Enter in the panel to open it with the default app.</p>
      {:else if file.video}
        <div class="stage">
          <!-- svelte-ignore a11y_media_has_caption (local files have no caption track) -->
          <video bind:this={video} {src} controls autoplay onloadedmetadata={videoReady} onerror={() => (broken = true)}
            style:width={shown ? `${shown.w}px` : null} style:height={shown ? `${shown.h}px` : null}
            style:visibility={shown ? null : 'hidden'}></video>
        </div>
      {:else}
        <div class="stage" {@attach (el) => { if (img) el.replaceChildren(img); }}></div>
      {/if}
    </div>
  {:else}
    <pre bind:this={bodyEl} tabindex="-1">{#if file.binary}Binary file — press F4 or Enter in the panel to open it with the default app.{:else}{file.content}{/if}</pre>
  {/if}
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
  .image {
    flex: 1;
    min-height: 0;
    overflow: auto;
    outline: none;
    cursor: zoom-out;
  }
  .image.fit { cursor: zoom-in; }
  .image.video { cursor: default; }
  /* Grows with the image, so centring never pushes part of it out of reach. */
  .stage {
    display: flex;
    align-items: center;
    justify-content: center;
    width: max-content;
    height: max-content;
    min-width: 100%;
    min-height: 100%;
  }
  .stage :global(img), video { display: block; flex: none; }
  .note { margin: 0; text-align: center; padding: 16px; color: var(--muted); }
</style>
