<script>
  /**
   * Jump — the answer to "where is that file?".
   *
   * The map alone cannot answer it once a repository has more than a screenful
   * of cards, so wayfinding gets its own surface rather than another filter.
   */
  import { SPRINGS, Spring, prefersReducedMotion } from './lib/motion.js';

  let { open = false, files = [], onpick = () => {}, onclose = () => {} } = $props();

  let query = $state('');
  let cursor = $state(0);
  let input = $state(null);
  let mounted = $state(false);
  let progress = $state(0);

  // Entering: the surface scales and its blur deepens together, so it reads as
  // a material arriving rather than an image fading up.
  const entrance = new Spring(0, {
    ...SPRINGS.ui,
    epsilon: 0.002,
    onchange: (value) => (progress = value),
    onsettle: () => {
      if (progress <= 0.01) mounted = false;
    }
  });

  $effect(() => {
    if (open && !mounted) {
      mounted = true;
      query = '';
      cursor = 0;
      entrance.jump(0);
      entrance.to(1, { damping: 0.86, response: 0.3 });
    } else if (!open && mounted) {
      // Out along the same path it came in on.
      entrance.to(0, SPRINGS.ui);
    }
  });

  $effect(() => {
    if (mounted && input) input.focus();
  });

  function score(file, needle) {
    const haystack = `${file.label} ${file.id}`.toLowerCase();
    const index = haystack.indexOf(needle);
    if (index >= 0) return 1000 - index;
    // Subsequence match, so "mapapi" still finds map_api.go.
    let at = 0;
    for (const character of needle) {
      at = haystack.indexOf(character, at);
      if (at === -1) return -1;
      at += 1;
    }
    return 100;
  }

  const results = $derived.by(() => {
    const needle = query.trim().toLowerCase();
    const ranked = needle
      ? files
          .map((file) => ({ file, rank: score(file, needle) }))
          .filter((entry) => entry.rank >= 0)
          .sort((left, right) => right.rank - left.rank || left.file.id.localeCompare(right.file.id))
          .map((entry) => entry.file)
      : [...files].sort((left, right) => (right.periodCommits ?? 0) - (left.periodCommits ?? 0));
    return ranked.slice(0, 40);
  });

  $effect(() => {
    if (cursor >= results.length) cursor = Math.max(0, results.length - 1);
  });

  function onKeyDown(event) {
    if (event.key === 'ArrowDown') cursor = Math.min(results.length - 1, cursor + 1);
    else if (event.key === 'ArrowUp') cursor = Math.max(0, cursor - 1);
    else if (event.key === 'Enter' && results[cursor]) onpick(results[cursor].id);
    else if (event.key === 'Escape') onclose();
    else return;
    event.preventDefault();
  }
</script>

{#if mounted}
  <div
    class="scrim"
    class:reduced={prefersReducedMotion()}
    style="--p:{progress}"
    role="presentation"
    onpointerdown={onclose}
  >
    <div
      class="palette"
      role="dialog"
      tabindex="-1"
      aria-modal="true"
      aria-label="Jump to a file"
      onpointerdown={(event) => event.stopPropagation()}
    >
      <input
        bind:this={input}
        bind:value={query}
        onkeydown={onKeyDown}
        placeholder="Jump to a file"
        aria-label="Jump to a file"
        spellcheck="false"
        autocomplete="off"
      />
      <ul>
        {#each results as file, index (file.id)}
          <li>
            <button
              class:active={index === cursor}
              onpointerenter={() => (cursor = index)}
              onclick={() => onpick(file.id)}
            >
              <span class="name t-title">{file.label}</span>
              <span class="path t-mono">{file.id}</span>
              {#if file.change}<em class="flag">changing</em>{/if}
              <b class="t-mono">{file.periodCommits ?? 0}</b>
            </button>
          </li>
        {:else}
          <li class="empty t-small">Nothing matches “{query}”.</li>
        {/each}
      </ul>
      <footer class="t-small">
        <span><kbd>↑</kbd><kbd>↓</kbd> move</span>
        <span><kbd>↵</kbd> focus on the map</span>
        <span><kbd>esc</kbd> back</span>
      </footer>
    </div>
  </div>
{/if}

<style>
  .scrim {
    position: fixed;
    inset: 0;
    z-index: 60;
    display: grid;
    justify-items: center;
    align-content: start;
    padding: min(16vh, 140px) 20px 20px;
    /* A modal task dims what it interrupts; a parallel panel would not. */
    background: color-mix(in srgb, var(--ground-deep) 46%, transparent);
    opacity: var(--p);
  }

  .palette {
    width: min(600px, 100%);
    max-height: min(560px, 70vh);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid var(--hairline);
    border-radius: 16px;
    background: var(--material-thick);
    backdrop-filter: blur(calc(var(--material-blur) * var(--p))) saturate(180%);
    box-shadow: var(--shadow-sheet);
    transform: scale(calc(0.94 + 0.06 * var(--p)));
    transform-origin: top center;
  }

  .scrim.reduced .palette {
    transform: none;
  }

  input {
    flex: none;
    padding: 15px 18px;
    border: 0;
    border-bottom: 1px solid var(--hairline);
    background: transparent;
    font-size: 1rem;
    letter-spacing: -0.01em;
    outline: none;
  }

  input::placeholder {
    color: var(--text-3);
  }

  ul {
    flex: 1 1 auto;
    min-height: 0;
    margin: 0;
    padding: 6px;
    overflow-y: auto;
    list-style: none;
  }

  li button {
    width: 100%;
    display: grid;
    grid-template-columns: minmax(0, auto) minmax(0, 1fr) auto auto;
    align-items: baseline;
    gap: 10px;
    padding: 8px 12px;
    border: 0;
    border-radius: var(--radius-chip);
    background: transparent;
    text-align: left;
    cursor: pointer;
  }

  li button.active {
    background: var(--accent-soft);
  }

  li button:active {
    transform: scale(0.995);
  }

  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .path {
    color: var(--text-3);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .flag {
    color: var(--warn);
    font-size: 0.66rem;
    font-style: normal;
    font-weight: 700;
  }

  li button b {
    color: var(--text-3);
    font-size: 0.7rem;
  }

  .empty {
    padding: 18px 14px;
    color: var(--text-3);
  }

  footer {
    flex: none;
    display: flex;
    gap: 16px;
    padding: 9px 16px;
    border-top: 1px solid var(--hairline);
    color: var(--text-3);
  }

  kbd {
    margin-right: 3px;
    padding: 1px 5px;
    border: 1px solid var(--hairline);
    border-radius: 5px;
    background: var(--surface);
    font: 0.68rem var(--mono);
  }
</style>
