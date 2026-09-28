<script lang="ts" module>
  export type Kind =
    | 'up' | 'folder' | 'app' | 'file' | 'image' | 'video' | 'audio' | 'archive'
    | 'code' | 'text' | 'pdf' | 'doc' | 'sheet' | 'slides' | 'font' | 'exec' | 'disk';

  const byExt: Record<string, Kind> = {};
  const groups: [Kind, string][] = [
    ['image', 'png jpg jpeg gif webp bmp tif tiff heic heif svg ico icns avif raw cr2 nef psd ai sketch fig'],
    ['video', 'mp4 mov mkv avi webm m4v wmv flv mpg mpeg 3gp'],
    ['audio', 'mp3 wav flac aac m4a ogg opus aiff aif wma mid midi'],
    ['archive', 'zip tar gz tgz bz2 xz zst 7z rar lz4 jar war'],
    ['disk', 'dmg iso img pkg deb rpm msi'],
    ['code', 'js mjs cjs ts tsx jsx go rs py rb java kt swift c h cc cpp hpp cs php lua sh bash zsh fish ps1 svelte vue html htm css scss sass less sql json jsonc yaml yml toml xml gradle mod sum lock make cmake dockerfile proto graphql wasm ipynb'],
    ['text', 'txt md markdown rst log csv tsv ini cfg conf env rtf tex org'],
    ['pdf', 'pdf'],
    ['doc', 'doc docx odt pages'],
    ['sheet', 'xls xlsx ods numbers'],
    ['slides', 'ppt pptx odp key'],
    ['font', 'ttf otf woff woff2'],
    ['exec', 'exe bin command out appimage bat cmd com'],
  ];
  for (const [kind, exts] of groups) for (const e of exts.split(' ')) byExt[e] = kind;

  // Extensionless names that are recognisably code or config.
  const byName: Record<string, Kind> = {};
  for (const n of 'dockerfile makefile justfile gemfile rakefile procfile .gitignore .gitattributes .gitmodules .editorconfig .npmrc .nvmrc .prettierrc .eslintrc .babelrc .dockerignore .env .bashrc .bash_profile .zshrc .zprofile .profile .vimrc .tmux.conf'.split(' ')) byName[n] = 'code';

  export function kindOf(entry: { name: string; ext: string; isDir: boolean; up?: boolean }): Kind {
    if (entry.up) return 'up';
    if (entry.isDir) return /\.app$/i.test(entry.name) ? 'app' : 'folder';
    return byExt[entry.ext] ?? byName[entry.name.toLowerCase()] ?? 'file';
  }
</script>

<script lang="ts">
  let { kind, link = false }: { kind: Kind; link?: boolean } = $props();

  const isFile = $derived(kind !== 'up' && kind !== 'folder' && kind !== 'app');
</script>

<svg class="icon k-{kind}" viewBox="0 0 24 24" aria-hidden="true">
  {#if kind === 'up'}
    <path class="line" d="M20 20h-7a4 4 0 0 1-4-4V4" />
    <path class="line" d="m4 9 5-5 5 5" />
  {:else if kind === 'folder'}
    <path class="body" d="M3 6.5A1.5 1.5 0 0 1 4.5 5h4.38a1.5 1.5 0 0 1 1.06.44L11.5 7h8A1.5 1.5 0 0 1 21 8.5v9a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 17.5z" />
    <path class="line faint" d="M3 10h18" />
  {:else if kind === 'app'}
    <rect class="body" x="3.5" y="3.5" width="17" height="17" rx="4.5" />
    <path class="line" d="M12 7.5v9M7.5 12h9" />
  {:else}
    <path class="body" d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z" />
    <path class="line faint" d="M14 3v4a1 1 0 0 0 1 1h4" />
    {#if kind === 'image'}
      <circle class="line" cx="10" cy="12.5" r="1.4" />
      <path class="line" d="m17 18-3.2-3.2a1 1 0 0 0-1.4 0L7.5 19.5" />
    {:else if kind === 'video'}
      <path class="mark" d="M10 11.2v5.6a.5.5 0 0 0 .75.43l4.5-2.8a.5.5 0 0 0 0-.86l-4.5-2.8a.5.5 0 0 0-.75.43z" />
    {:else if kind === 'audio'}
      <path class="line" d="M11 17.5V11l4.5-1.2v6" />
      <circle class="mark" cx="9.6" cy="17.5" r="1.5" />
      <circle class="mark" cx="14.1" cy="15.8" r="1.5" />
    {:else if kind === 'archive'}
      <path class="line" d="M11 5h1M12 7h1M11 9h1M12 11h1" />
      <rect class="line" x="10.5" y="13" width="3" height="4" rx=".8" />
    {:else if kind === 'disk'}
      <circle class="line" cx="12" cy="14.5" r="3.5" />
      <circle class="mark" cx="12" cy="14.5" r="1" />
    {:else if kind === 'code'}
      <path class="line" d="m10 12-2.5 2.5L10 17M14 12l2.5 2.5L14 17" />
    {:else if kind === 'text' || kind === 'doc'}
      <path class="line" d="M8.5 12h7M8.5 15h7M8.5 18h4" />
    {:else if kind === 'pdf'}
      <path class="line" d="M8.5 17.5c2-1.2 3.6-3.7 3.8-6.3.1-1.1-1.2-1.3-1.3-.2-.2 2.4 2.5 5.3 4.6 5.8 1 .3 1.3-.9.3-1-2.3-.3-5.2.5-7.4 1.7" />
    {:else if kind === 'sheet'}
      <path class="line" d="M8 11.5h8v7H8zM8 15h8M12 11.5v7" />
    {:else if kind === 'slides'}
      <rect class="line" x="8" y="11.5" width="8" height="5" rx=".8" />
      <path class="line" d="M12 16.5v2" />
    {:else if kind === 'font'}
      <path class="line" d="m9 18 3-7 3 7M10.2 15.5h3.6" />
    {:else if kind === 'exec'}
      <path class="line" d="m8.5 12.5 2.5 2-2.5 2M12.5 17.5h3" />
    {/if}
  {/if}
  {#if link}
    <circle class="badge" cx="6" cy="18" r="4.2" />
    <path class="badge-arrow" d="M4.4 19.4 7.4 16.5M5.2 16.4h2.3v2.3" />
  {/if}
</svg>

<style>
  .icon {
    width: 16px;
    height: 16px;
    flex: none;
    overflow: visible;
    color: var(--ic, var(--ic-file));
    fill: none;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .body {
    fill: color-mix(in srgb, currentColor 16%, transparent);
    stroke: currentColor;
    stroke-width: 1.6;
  }
  .line { stroke: currentColor; stroke-width: 1.6; }
  .faint { opacity: 0.55; }
  .mark { fill: currentColor; }
  .badge { fill: var(--panel); stroke: currentColor; stroke-width: 1.3; }
  .badge-arrow { stroke: currentColor; stroke-width: 1.3; }

  .k-folder { --ic: var(--ic-folder); }
  .k-folder .body { fill: color-mix(in srgb, currentColor 34%, transparent); }
  .k-app { --ic: var(--ic-app); }
  .k-up { --ic: var(--muted); }
  .k-image { --ic: var(--ic-image); }
  .k-video { --ic: var(--ic-video); }
  .k-audio { --ic: var(--ic-audio); }
  .k-archive, .k-disk { --ic: var(--ic-archive); }
  .k-code { --ic: var(--ic-code); }
  .k-text { --ic: var(--ic-text); }
  .k-pdf { --ic: var(--ic-pdf); }
  .k-doc { --ic: var(--ic-doc); }
  .k-sheet { --ic: var(--ic-sheet); }
  .k-slides { --ic: var(--ic-slides); }
  .k-font, .k-exec { --ic: var(--ic-code); }
</style>
