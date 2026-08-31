<script>
  /**
   * The map's one primary control. The indicator is sprung rather than
   * transitioned so that rattling between options stays continuous instead of
   * restarting a fixed animation on every press.
   */
  import { SPRINGS, Spring, prefersReducedMotion } from './lib/motion.js';

  let { options = [], value = '', onchange = () => {}, label = '' } = $props();

  let container = $state(null);
  let left = $state(0);
  let width = $state(0);
  let ready = $state(false);

  const springX = new Spring(0, { ...SPRINGS.ui, onchange: (v) => (left = v) });
  const springW = new Spring(0, { ...SPRINGS.ui, onchange: (v) => (width = v) });

  function measure() {
    if (!container) return;
    const active = container.querySelector('[aria-selected="true"]');
    if (!active) return;
    const target = { left: active.offsetLeft, width: active.offsetWidth };
    if (!ready) {
      springX.jump(target.left);
      springW.jump(target.width);
      ready = true;
      return;
    }
    springX.to(target.left, SPRINGS.ui);
    springW.to(target.width, SPRINGS.ui);
  }

  $effect(() => {
    // Re-read on both a selection change and a resize.
    value;
    options;
    measure();
    if (!container) return;
    const observer = new ResizeObserver(measure);
    observer.observe(container);
    return () => observer.disconnect();
  });
</script>

<div class="segmented" bind:this={container} role="tablist" aria-label={label}>
  <span class="indicator" class:ready class:reduced={prefersReducedMotion()} style="transform:translateX({left}px);width:{width}px"></span>
  {#each options as option (option.id)}
    <button
      role="tab"
      aria-selected={value === option.id}
      title={option.hint ?? option.label}
      onclick={() => onchange(option.id)}
    >
      {option.label}
    </button>
  {/each}
</div>

<style>
  .segmented {
    position: relative;
    display: flex;
    gap: 2px;
    padding: 3px;
    border: 1px solid var(--hairline);
    border-radius: 12px;
    background: var(--material);
    backdrop-filter: blur(var(--material-blur)) saturate(180%);
    box-shadow: var(--shadow-card);
  }

  .indicator {
    position: absolute;
    top: 3px;
    bottom: 3px;
    left: 0;
    border-radius: 9px;
    background: var(--surface);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.14);
    opacity: 0;
  }

  .indicator.ready {
    opacity: 1;
  }

  button {
    position: relative;
    z-index: 1;
    padding: 6px 13px;
    border: 0;
    border-radius: 9px;
    background: transparent;
    color: var(--text-2);
    font-size: 0.79rem;
    font-weight: 580;
    letter-spacing: -0.004em;
    white-space: nowrap;
    cursor: pointer;
    transition: color 140ms ease, transform 90ms ease;
  }

  button[aria-selected='true'] {
    color: var(--text);
    font-weight: 640;
  }

  /* Feedback on the press itself, before the selection commits. */
  button:active {
    transform: scale(0.965);
  }

  button:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
</style>
