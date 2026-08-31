<script>
  import { Handle, Position } from '@xyflow/svelte';
  import { areaColors } from './lib/architecture.js';

  let { data, selected = false } = $props();

  const statusNames = { A: 'added', M: 'modified', D: 'deleted', R: 'renamed' };

  let isScope = $derived(data.kind === 'module');
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
  class:tests={data.isTest}
  class:changed={Boolean(data.change)}
  class:affected={data.affected}
  data-status={data.change?.status ?? ''}
  data-area={data.area ?? ''}
  style:--card-accent={areaColors[data.area] ?? areaColors.unknown}
  onpointerdown={() => (pressed = true)}
  onpointerup={() => (pressed = false)}
  onpointerleave={() => (pressed = false)}
  onpointercancel={() => (pressed = false)}
>
  <header>
    <span class="t-label kind">{isScope ? data.projectName : data.language || 'source'}</span>
    {#if isScope && data.isTest}<strong class="badge linked">Tests</strong>{/if}
    {#if data.context}<strong class="badge linked">Related</strong>{/if}
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

  {#if isScope}
    <footer><span class="t-mono">{data.entryPoints.length ? `${data.entryPoints.length} entry points` : data.isTest ? 'Tests and helpers' : 'Static dependencies'}</span></footer>
  {:else}
  <footer>
    <div class="rail" aria-hidden="true"><span style="width:{railWidth}%"></span></div>
    <span class="t-mono count">{commits}</span>
    {#if data.change}
      <span class="t-mono diff"><b>+{data.change.additions}</b> <i>−{data.change.deletions}</i></span>
    {/if}
  </footer>
  {/if}
</article>
<Handle type="source" position={Position.Bottom} />

<style>
  .card.tests { border-style: dashed; }
  .card.scope { min-height: 155px; }
  .card {
    position: relative;
    width: 234px;
    padding: 12px 14px 11px;
    border: 1px solid color-mix(in srgb, var(--card-accent) 55%, var(--hairline));
    border-radius: var(--radius-card);
    background: color-mix(in srgb, var(--card-accent) 7%, var(--surface));
    color: var(--text);
    box-shadow: var(--shadow-card);
    /* Short, cheap, compositor-only. Anything a finger drives is sprung in JS
       instead -- these are discrete state changes, not gestures. */
    transition: transform 130ms cubic-bezier(0.2, 0, 0, 1), box-shadow 150ms ease,
      border-color 150ms ease, opacity 180ms ease;
  }

  .card:hover {
    border-color: var(--card-accent);
    box-shadow: var(--shadow-lift);
    transform: translateY(-1px);
  }

  /* Feedback lands on pointer-down, not on the click that follows it. */
  .card.pressed {
    transform: scale(0.978);
    transition-duration: 90ms;
  }

  .card.selected,
  .card.scope.selected {
    border-color: var(--card-accent);
    box-shadow: 0 0 0 3px color-mix(in srgb, var(--card-accent) 18%, transparent), var(--shadow-lift);
    transform: translateY(-2px);
  }

  /* With something selected, its neighbours stay legible and the rest recede.
     Nothing is hidden -- the map keeps its shape. */
  .card.faded {
    opacity: 0.32;
  }

  .card.linked {
    border-color: color-mix(in srgb, var(--card-accent) 75%, var(--hairline));
  }

  .card.scope {
    width: 250px;
    background: color-mix(in srgb, var(--card-accent) 12%, var(--surface));
    border-color: color-mix(in srgb, var(--card-accent) 60%, var(--hairline));
  }

  .card.affected {
    border-style: dashed;
  }

  header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    min-height: 15px;
  }

  .kind {
    color: var(--card-accent);
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
    color: var(--card-accent);
    background: color-mix(in srgb, var(--card-accent) 16%, transparent);
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
    background: var(--card-accent);
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
