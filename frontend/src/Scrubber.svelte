<script>
  /**
   * The time window, as a thing you drag rather than three buttons.
   *
   * The handle is the left edge of the window, sitting directly on the commit
   * history it selects — the control and what it changes are the same object,
   * so nothing needs a label to explain the mapping. Only three windows are
   * honestly available from the data, so the track is detented at three stops
   * and a flick lands on whichever one the throw was heading for.
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

  let { buckets = [], period = '30', onperiod = () => {} } = $props();

  // Weeks of history each window covers, used to place its detent on the track.
  const WINDOWS = [
    { id: '30', label: '30 days', weeks: 30 / 7 },
    { id: '90', label: '90 days', weeks: 90 / 7 },
    { id: 'all', label: 'all time', weeks: Infinity }
  ];

  let track = $state(0);
  let handle = $state(0);
  let dragging = $state(false);
  let element = $state(null);
  // The last window this component itself asked for, so the effect that follows
  // an external change does not re-target -- and flatten -- our own release
  // spring the moment the parent echoes the value back.
  let selfPeriod = null;

  const weeks = $derived(Math.max(1, buckets.length));
  const peak = $derived(Math.max(1, ...buckets.map((bucket) => bucket.count)));

  function stopFor(id) {
    const window = WINDOWS.find((entry) => entry.id === id) ?? WINDOWS[0];
    const covered = Math.min(weeks, window.weeks);
    return track * (1 - covered / weeks);
  }

  const stops = $derived(WINDOWS.map((window) => stopFor(window.id)));
  // Live during a drag: the map updates as the handle moves, not on release.
  const activeId = $derived(WINDOWS[stops.indexOf(nearest(stops, handle))]?.id ?? '30');
  const covered = $derived(track > 0 ? 1 - handle / track : 1);

  const spring = new Spring(0, {
    ...SPRINGS.momentum,
    onchange: (value) => (handle = value)
  });

  function measure(width = element?.clientWidth ?? 0) {
    if (width === track) return;
    track = width;
    if (!dragging) spring.jump(stopFor(period));
  }

  $effect(() => {
    if (!element) return;
    // Measure straight away rather than waiting for the observer's first
    // delivery: until the track has a width every stop collapses onto zero and
    // the handle has nowhere to go.
    measure();
    const observer = new ResizeObserver(([entry]) => measure(entry.contentRect.width));
    observer.observe(element);
    return () => observer.disconnect();
  });

  $effect(() => {
    // Follow a period change from elsewhere without fighting a drag in
    // progress, or overwriting the spring we just handed a release velocity.
    if (dragging || track <= 0 || period === selfPeriod) return;
    spring.to(stopFor(period), SPRINGS.ui);
  });

  function announce(id) {
    if (id === selfPeriod) return;
    selfPeriod = id;
    onperiod(id);
  }

  const tracker = velocityTracker();
  let pointerId = null;
  let grabOffset = 0;

  function onPointerDown(event) {
    pointerId = event.pointerId;
    element.setPointerCapture(pointerId);
    spring.hold();
    // Respect where it was grabbed. Jumping the handle under the finger is the
    // fastest way to break the feeling that you are holding a real thing.
    const bounds = element.getBoundingClientRect();
    measure(bounds.width);
    const pointer = event.clientX - bounds.left;
    grabOffset = Math.abs(pointer - handle) < 26 ? pointer - handle : 0;
    if (grabOffset === 0) spring.set(clamp(pointer, 0, track));
    dragging = true;
    tracker.reset(handle, event.timeStamp);
  }

  function onPointerMove(event) {
    if (pointerId !== event.pointerId || !dragging) return;
    const bounds = element.getBoundingClientRect();
    let next = event.clientX - bounds.left - grabOffset;
    if (next < 0) next = rubberband(next, track);
    else if (next > track) next = track + rubberband(next - track, track);
    spring.set(next);
    tracker.push(next, event.timeStamp);
    // The map re-reads its numbers as the handle moves, not when it is let go.
    announce(activeId);
  }

  function onPointerUp(event) {
    if (pointerId !== event.pointerId) return;
    element.releasePointerCapture(pointerId);
    pointerId = null;
    dragging = false;
    const velocity = tracker.velocity();
    const landing = nearest(stops, clamp(handle + project(velocity), 0, track));
    spring.to(landing, { ...SPRINGS.momentum, velocity });
    announce(WINDOWS[stops.indexOf(landing)]?.id ?? '30');
  }

  function step(direction) {
    const index = WINDOWS.findIndex((window) => window.id === period);
    selfPeriod = null;
    onperiod(WINDOWS[clamp(index + direction, 0, WINDOWS.length - 1)].id);
  }

  function onKeyDown(event) {
    if (event.key === 'ArrowLeft') step(1);
    else if (event.key === 'ArrowRight') step(-1);
    else return;
    event.preventDefault();
  }

  const current = $derived(WINDOWS.find((window) => window.id === activeId) ?? WINDOWS[0]);
  // "all time" already reads as a span; "last all time" does not.
  const currentLabel = $derived(current.id === 'all' ? 'all time' : `last ${current.label}`);
</script>

<div class="scrubber" class:dragging class:reduced={prefersReducedMotion()}>
  <div class="caption">
    <span class="t-label">Activity window</span>
    <strong class="t-small">{currentLabel}</strong>
  </div>

  <div
    class="track"
    bind:this={element}
    role="slider"
    tabindex="0"
    aria-label="Activity window"
    aria-valuetext={currentLabel}
    aria-valuemin="0"
    aria-valuemax="2"
    aria-valuenow={WINDOWS.findIndex((window) => window.id === period)}
    onpointerdown={onPointerDown}
    onpointermove={onPointerMove}
    onpointerup={onPointerUp}
    onpointercancel={onPointerUp}
    onkeydown={onKeyDown}
  >
    <div class="bars" aria-hidden="true">
      {#each buckets as bucket, index}
        <i
          class:inside={index / weeks >= 1 - covered}
          style="height:{Math.max(9, Math.round((bucket.count / peak) * 100))}%"
          title="{bucket.count} commits"
        ></i>
      {/each}
    </div>
    <div class="window" style="left:{handle}px" aria-hidden="true">
      <span class="knob"></span>
    </div>
  </div>
</div>

<style>
  .scrubber {
    display: grid;
    gap: 6px;
    width: min(420px, 46vw);
    padding: 9px 12px 10px;
    border: 1px solid var(--hairline);
    border-radius: 14px;
    background: var(--material);
    backdrop-filter: blur(var(--material-blur)) saturate(180%);
    box-shadow: var(--shadow-card);
  }

  .caption {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 10px;
    color: var(--text-3);
  }

  .caption strong {
    color: var(--text);
    font-weight: 620;
    font-variant-numeric: tabular-nums;
  }

  .track {
    position: relative;
    height: 40px;
    cursor: ew-resize;
    touch-action: none;
    outline: none;
  }

  .track:focus-visible {
    border-radius: 6px;
    box-shadow: 0 0 0 2px var(--accent);
  }

  .bars {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: flex-end;
    gap: 2px;
  }

  .bars i {
    flex: 1 1 0;
    min-width: 0;
    border-radius: 2px 2px 1px 1px;
    background: var(--hairline-strong);
    transition: background-color 130ms ease;
  }

  .bars i.inside {
    background: var(--accent);
  }

  .dragging .bars i {
    transition: none;
  }

  .window {
    position: absolute;
    top: -3px;
    bottom: -3px;
    width: 2px;
    background: var(--text);
    border-radius: 2px;
  }

  .knob {
    position: absolute;
    left: 50%;
    top: 50%;
    width: 15px;
    height: 22px;
    transform: translate(-50%, -50%);
    border: 1px solid var(--hairline-strong);
    border-radius: 6px;
    background: var(--surface);
    box-shadow: var(--shadow-card);
  }

  /* Grabbing something small should make it easier to keep hold of. */
  .dragging .knob {
    transform: translate(-50%, -50%) scale(1.12);
  }

  .reduced .knob,
  .reduced .bars i {
    transition: none;
  }

  @media (max-width: 720px) {
    .scrubber {
      width: 100%;
    }
  }
</style>
