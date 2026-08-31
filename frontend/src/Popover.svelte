<script>
  /**
   * A small surface that grows out of the control that opened it, so the
   * relationship between the button and its contents needs no explaining.
   */
  import { SPRINGS, Spring, prefersReducedMotion } from './lib/motion.js';

  let { open = false, align = 'end', label = '', onclose = () => {}, children } = $props();

  let mounted = $state(false);
  let progress = $state(0);

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
      entrance.jump(0);
      entrance.to(1, { damping: 0.85, response: 0.26 });
    } else if (!open && mounted) {
      entrance.to(0, { damping: 1, response: 0.2 });
    }
  });

  $effect(() => {
    if (!mounted) return;
    const dismiss = (event) => {
      if (!event.target.closest?.('.popover, [data-popover-trigger]')) onclose();
    };
    const escape = (event) => event.key === 'Escape' && onclose();
    window.addEventListener('pointerdown', dismiss, true);
    window.addEventListener('keydown', escape);
    return () => {
      window.removeEventListener('pointerdown', dismiss, true);
      window.removeEventListener('keydown', escape);
    };
  });
</script>

{#if mounted}
  <div
    class="popover {align}"
    class:reduced={prefersReducedMotion()}
    style="--p:{progress}"
    role="dialog"
    aria-label={label}
  >
    {@render children?.()}
  </div>
{/if}

<style>
  .popover {
    position: absolute;
    top: calc(100% + 8px);
    z-index: 40;
    min-width: 210px;
    max-width: min(340px, 80vw);
    padding: 11px 13px;
    border: 1px solid var(--hairline);
    border-radius: 13px;
    background: var(--material-thick);
    backdrop-filter: blur(calc(var(--material-blur) * var(--p))) saturate(180%);
    box-shadow: var(--shadow-lift);
    opacity: var(--p);
    /* Anchored to the trigger, not to its own centre. */
    transform: scale(calc(0.9 + 0.1 * var(--p)));
  }

  .popover.end {
    right: 0;
    transform-origin: top right;
  }

  .popover.start {
    left: 0;
    transform-origin: top left;
  }

  .reduced {
    transform: none;
  }
</style>
