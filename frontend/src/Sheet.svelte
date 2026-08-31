<script>
  /**
   * An inspector surface you can grab.
   *
   * The whole component exists to obey four rules: track the finger 1:1, stay
   * grabbable while it is still moving, land where the flick was *going* rather
   * than where it was released, and leave along the path it arrived on.
   *
   * `extent` is how many pixels of the sheet are revealed. The content box is
   * always the full size and slides, so nothing reflows during a drag.
   */
  import {
    SPRINGS,
    Spring,
    clamp,
    nearest,
    prefersReducedMotion,
    project,
    rubberband,
    velocityTracker
  } from './lib/motion.js';

  let {
    open = false,
    axis = 'x',
    detents = [420],
    label = 'Inspector',
    onclose = () => {},
    onextent = () => {},
    header,
    children
  } = $props();

  const maxDetent = $derived(Math.max(...detents));
  const minDetent = $derived(Math.min(...detents));

  let extent = $state(0);
  let dragging = $state(false);
  let mounted = $state(false);
  let surface = $state(null);

  const spring = new Spring(0, {
    ...SPRINGS.sheet,
    onchange: (value) => {
      extent = value;
      onextent(value);
    },
    onsettle: () => {
      if (extent <= 0.5) mounted = false;
    }
  });

  // How far past its smallest detent the sheet is, 0..1. Drives the material:
  // a glass surface should thicken as it arrives, not just fade in.
  const presence = $derived(clamp(extent / Math.max(1, minDetent), 0, 1));
  const expanded = $derived(extent >= maxDetent - 12);

  $effect(() => {
    if (open && !mounted) {
      mounted = true;
      spring.jump(0);
      spring.to(minDetent, SPRINGS.sheet);
    } else if (!open && mounted) {
      spring.to(0, SPRINGS.ui);
    }
  });

  // --- Gesture -----------------------------------------------------------

  const tracker = velocityTracker();
  let pointerId = null;
  let startPointer = 0;
  let startExtent = 0;

  function pointerAxis(event) {
    return axis === 'y' ? event.clientY : event.clientX;
  }

  function onPointerDown(event) {
    if (event.button !== undefined && event.button !== 0) return;
    if (event.target.closest('button, a, input')) return;
    pointerId = event.pointerId;
    surface.setPointerCapture(pointerId);
    // Take the value that is on screen right now, velocity and all. A sheet
    // caught mid-flight must follow the finger from where it visibly is.
    spring.hold();
    startPointer = pointerAxis(event);
    startExtent = extent;
    dragging = true;
    tracker.reset(extent, event.timeStamp);
  }

  function onPointerMove(event) {
    if (pointerId !== event.pointerId || !dragging) return;
    // Both axes reveal the sheet by moving *against* the axis direction: up
    // for a bottom sheet, left for a right-hand panel.
    const travelled = startPointer - pointerAxis(event);
    let next = startExtent + travelled;
    if (next > maxDetent) {
      next = maxDetent + rubberband(next - maxDetent, maxDetent);
    }
    next = Math.max(0, next);
    spring.set(next);
    tracker.push(next, event.timeStamp);
  }

  function onPointerUp(event) {
    if (pointerId !== event.pointerId) return;
    surface.releasePointerCapture(pointerId);
    pointerId = null;
    dragging = false;

    const velocity = tracker.velocity();
    const projected = extent + project(velocity);
    // Direction of the throw decides dismissal, not where the finger stopped.
    const thrownShut = velocity < -650;
    const driftedShut = projected < minDetent * 0.55;

    if (thrownShut || driftedShut) {
      spring.to(0, { ...SPRINGS.sheet, velocity });
      onclose();
      return;
    }
    // Hand the finger's exact speed to the spring so there is no seam between
    // dragging and settling.
    spring.to(nearest(detents, projected), { ...SPRINGS.sheet, velocity });
  }

  function cycle(direction) {
    const index = detents.indexOf(nearest(detents, extent));
    const next = detents[clamp(index + direction, 0, detents.length - 1)];
    if (next === extent && direction < 0) {
      onclose();
      return;
    }
    spring.to(next, SPRINGS.ui);
  }

  function onKeyDown(event) {
    const grow = axis === 'y' ? 'ArrowUp' : 'ArrowLeft';
    const shrink = axis === 'y' ? 'ArrowDown' : 'ArrowRight';
    if (event.key === grow) cycle(1);
    else if (event.key === shrink) cycle(-1);
    else if (event.key === 'Enter' || event.key === ' ') cycle(expanded ? -1 : 1);
    else return;
    event.preventDefault();
  }

  const style = $derived(
    axis === 'y'
      ? `height:${maxDetent}px;transform:translate3d(0,${maxDetent - extent}px,0)`
      : `width:${maxDetent}px;transform:translate3d(${maxDetent - extent}px,0,0)`
  );
</script>

{#if mounted}
  <section
    class="sheet {axis}"
    class:dragging
    class:expanded
    class:reduced={prefersReducedMotion()}
    style="{style};--presence:{presence}"
    aria-label={label}
  >
    <div
      class="grab"
      bind:this={surface}
      role="button"
      tabindex="0"
      aria-label="{label} size. Arrow keys resize, Escape closes."
      onpointerdown={onPointerDown}
      onpointermove={onPointerMove}
      onpointerup={onPointerUp}
      onpointercancel={onPointerUp}
      onkeydown={onKeyDown}
    >
      {#if axis === 'y'}<span class="grabber"></span>{/if}
      {@render header?.()}
    </div>
    <div class="body" class:locked={!expanded && detents.length > 1}>
      {@render children?.()}
    </div>
  </section>
{/if}

<style>
  .sheet {
    position: absolute;
    z-index: 30;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    /* The material thickens as the sheet arrives rather than fading in flat. */
    background: var(--material-thick);
    backdrop-filter: blur(calc(var(--material-blur) * var(--presence))) saturate(180%);
    border: 1px solid var(--hairline);
    box-shadow: var(--shadow-sheet);
    touch-action: none;
  }

  .sheet.x {
    top: 0;
    right: 0;
    bottom: 0;
    border-right: 0;
    border-radius: var(--radius-sheet) 0 0 var(--radius-sheet);
  }

  .sheet.y {
    left: 0;
    right: 0;
    bottom: 0;
    border-bottom: 0;
    border-radius: var(--radius-sheet) var(--radius-sheet) 0 0;
  }

  /* No motion preference: the surface cross-fades in place instead of sliding. */
  .sheet.reduced {
    transform: none !important;
    opacity: var(--presence);
    transition: opacity 180ms ease;
  }

  .grab {
    flex: none;
    padding: 0;
    border: 0;
    background: transparent;
    text-align: left;
    cursor: grab;
    touch-action: none;
    /* No highlight suppression games -- the header press state is real. */
    -webkit-user-select: none;
    user-select: none;
  }

  .sheet.x .grab {
    cursor: ew-resize;
  }

  .grab:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: -3px;
  }

  .dragging .grab {
    cursor: grabbing;
  }

  .grabber {
    display: block;
    width: 38px;
    height: 5px;
    margin: 8px auto 2px;
    border-radius: 999px;
    background: var(--hairline-strong);
  }

  .body {
    flex: 1 1 auto;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  /* Below the tallest detent the body's overflow is off-screen, so scrolling it
     would move content the reader cannot see. Drag up first. */
  .body.locked {
    overflow: hidden;
  }
</style>
