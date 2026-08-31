<script>
  import { Handle, Position } from '@xyflow/svelte';

  let { data, selected = false } = $props();

  const statusNames = { A: 'added', M: 'modified', D: 'deleted', R: 'renamed' };

  let isScope = $derived(String(data.id ?? '').startsWith('__'));
  let commits = $derived(data.periodCommits ?? 0);
  // Relative to the busiest file on screen, so the rail means something across
  // the whole map rather than against an arbitrary ceiling.
  let railWidth = $derived(commits === 0 ? 0 : Math.max(6, Math.round((data.intensity ?? 0) * 100)));
  let pressed = $state(false);
</script>

<Handle type="target" position={Position.Top} />
<article
  class="card {data.relation ?? 'none'}"
  class:selected
  class:pressed
  class:scope={isScope}
  class:changed={Boolean(data.change)}
  class:affected={data.affected}
  data-status={data.change?.status ?? ''}
  data-area={data.area ?? ''}
  onpointerdown={() => (pressed = true)}
  onpointerup={() => (pressed = false)}
  onpointerleave={() => (pressed = false)}
  onpointercancel={() => (pressed = false)}
>
  <header>
    <span class="t-label kind">{data.language || 'source'}</span>
    {#if data.change}
      <strong class="badge status" title={statusNames[data.change.status]}>
        {statusNames[data.change.status] ?? data.change.status}
      </strong>
    {:else if data.affected}
      <strong class="badge linked">linked</strong>
    {/if}
  </header>

  <h3 class="t-title">{data.label}</h3>
  <p class="t-small">{data.change?.summary?.changed ?? data.description}</p>

  <footer>
    <div class="rail" aria-hidden="true"><span style="width:{railWidth}%"></span></div>
    <span class="t-mono count">{commits}</span>
    {#if data.change}
      <span class="t-mono diff"><b>+{data.change.additions}</b> <i>−{data.change.deletions}</i></span>
    {/if}
  </footer>
</article>
<Handle type="source" position={Position.Bottom} />

<style>
  .card {
    position: relative;
    width: 234px;
    padding: 12px 14px 11px;
    border: 1px solid var(--hairline);
    border-radius: var(--radius-card);
    background: var(--surface);
    color: var(--text);
    box-shadow: var(--shadow-card);
    /* Short, cheap, compositor-only. Anything a finger drives is sprung in JS
       instead -- these are discrete state changes, not gestures. */
    transition: transform 130ms cubic-bezier(0.2, 0, 0, 1), box-shadow 150ms ease,
      border-color 150ms ease, opacity 180ms ease;
  }

  .card:hover {
    border-color: var(--hairline-strong);
    box-shadow: var(--shadow-lift);
    transform: translateY(-1px);
  }

  /* Feedback lands on pointer-down, not on the click that follows it. */
  .card.pressed {
    transform: scale(0.978);
    transition-duration: 90ms;
  }

  .card.selected {
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-soft), var(--shadow-lift);
    transform: translateY(-2px);
  }

  /* With something selected, its neighbours stay legible and the rest recede.
     Nothing is hidden -- the map keeps its shape. */
  .card.faded {
    opacity: 0.32;
  }

  .card.linked {
    border-color: color-mix(in srgb, var(--accent) 45%, var(--hairline));
  }

  .card.scope {
    width: 250px;
    background: color-mix(in srgb, var(--accent) 7%, var(--surface));
    border-color: color-mix(in srgb, var(--accent) 30%, var(--hairline));
  }

  .card.scope[data-area='backend'] {
    background: color-mix(in srgb, var(--bridge) 9%, var(--surface));
    border-color: color-mix(in srgb, var(--bridge) 32%, var(--hairline));
  }

  .card.affected {
    border-style: dashed;
  }

  .card[data-status='A'] {
    border-color: color-mix(in srgb, var(--add) 55%, var(--hairline));
  }

  .card[data-status='D'] {
    border-color: color-mix(in srgb, var(--del) 55%, var(--hairline));
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-height: 15px;
  }

  .kind {
    color: var(--text-3);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .badge {
    flex: none;
    padding: 2px 6px;
    border-radius: 999px;
    font-size: 0.62rem;
    font-weight: 700;
    letter-spacing: 0.03em;
  }

  .badge.status {
    color: var(--warn);
    background: color-mix(in srgb, var(--warn) 16%, transparent);
  }

  .card[data-status='A'] .badge.status {
    color: var(--add);
    background: color-mix(in srgb, var(--add) 16%, transparent);
  }

  .card[data-status='D'] .badge.status {
    color: var(--del);
    background: color-mix(in srgb, var(--del) 16%, transparent);
  }

  .badge.linked {
    color: var(--accent);
    background: var(--accent-soft);
  }

  h3 {
    margin: 7px 0 3px;
    overflow-wrap: anywhere;
  }

  p {
    margin: 0;
    color: var(--text-2);
    display: -webkit-box;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  footer {
    display: flex;
    align-items: center;
    gap: 9px;
    margin-top: 11px;
  }

  .rail {
    flex: 1 1 auto;
    height: 4px;
    overflow: hidden;
    border-radius: 999px;
    background: var(--hairline);
  }

  .rail span {
    display: block;
    height: 100%;
    border-radius: inherit;
    background: var(--accent);
    /* The rail follows the time scrubber continuously while it is dragged. */
    transition: width 120ms linear;
  }

  .count {
    flex: none;
    color: var(--text-3);
    font-size: 0.68rem;
  }

  .diff {
    flex: none;
    font-size: 0.68rem;
  }

  .diff b {
    color: var(--add);
  }

  .diff i {
    color: var(--del);
    font-style: normal;
  }

  .card.scope .rail,
  .card.scope .count {
    display: none;
  }

  :global(.svelte-flow__handle) {
    width: 6px;
    height: 6px;
    border: 0;
    opacity: 0;
  }

  @media (prefers-reduced-motion: reduce) {
    .card {
      transition: opacity 140ms ease, border-color 140ms ease;
      transform: none !important;
    }
    .rail span {
      transition: none;
    }
  }
</style>
