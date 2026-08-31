/**
 * Motion primitives, after Apple's "Designing Fluid Interfaces" (WWDC 2018).
 *
 * Everything here exists so that on-screen motion can start from the value that
 * is *currently rendered*, inherit the velocity the finger was carrying, and be
 * grabbed and reversed at any instant. Durations and keyframes cannot do that,
 * which is why there are none in this file.
 */

// --- Shared clock --------------------------------------------------------
// Every live spring steps on the same frame, so a visual, a sound and a haptic
// triggered together actually land together instead of drifting apart.

const live = new Set();
let frame = 0;
let lastFrameTime = 0;

function tick(now) {
  frame = 0;
  const dt = Math.min(0.064, (now - lastFrameTime) / 1000) || 1 / 60;
  lastFrameTime = now;
  // Snapshot: a spring may settle (and unregister) inside its own step.
  for (const spring of [...live]) spring._step(dt);
  if (live.size) schedule();
}

function schedule() {
  if (frame) return;
  lastFrameTime = performance.now();
  frame = requestAnimationFrame(tick);
}

export function prefersReducedMotion() {
  return typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches;
}

// --- Spring --------------------------------------------------------------
// Parameterised the way Apple parameterises it for designers: a damping ratio
// (how much it overshoots) and a response (how quickly it gets there), not the
// mass/stiffness/damping triplet. There is no duration -- settle time is an
// outcome of the parameters, not an input.

export const SPRINGS = {
  /** Default UI motion. Critically damped: arrives and stops, never bounces. */
  ui: { damping: 1, response: 0.35 },
  /** Repositioning something large, e.g. flying the camera. */
  move: { damping: 1, response: 0.4 },
  /** Only after a gesture carried momentum into it -- a flick, a throw. */
  momentum: { damping: 0.8, response: 0.34 },
  /** Sheets and drawers released from a drag. */
  sheet: { damping: 0.82, response: 0.3 }
};

export class Spring {
  constructor(value = 0, options = {}) {
    this.value = value;
    this.target = value;
    this.velocity = 0;
    this.damping = options.damping ?? SPRINGS.ui.damping;
    this.response = options.response ?? SPRINGS.ui.response;
    // Below this the motion is smaller than a pixel of perceivable change.
    this.epsilon = options.epsilon ?? 0.04;
    this.onchange = options.onchange ?? null;
    this.onsettle = options.onsettle ?? null;
    this.settled = true;
  }

  get moving() {
    return !this.settled;
  }

  /**
   * Re-target. The current value and velocity are deliberately preserved, so a
   * spring caught mid-flight blends into its new destination instead of cutting
   * to it -- this is what stops a reversal feeling like a brick wall.
   */
  to(target, options = {}) {
    if (options.damping !== undefined) this.damping = options.damping;
    if (options.response !== undefined) this.response = options.response;
    if (options.velocity !== undefined) this.velocity = options.velocity;
    this.target = target;
    if (prefersReducedMotion()) return this.jump(target);
    if (this.value === target && Math.abs(this.velocity) < this.epsilon) return this.jump(target);
    this.settled = false;
    live.add(this);
    schedule();
    return this;
  }

  /** Cut straight to a value. Used for first paint, never mid-interaction. */
  jump(value = this.target) {
    live.delete(this);
    this.value = value;
    this.target = value;
    this.velocity = 0;
    const wasMoving = !this.settled;
    this.settled = true;
    this.onchange?.(this.value, this);
    if (wasMoving) this.onsettle?.(this);
    return this;
  }

  /**
   * Hand control to a finger. The spring stops driving but keeps its velocity,
   * so a card grabbed while still flying carries that motion into the drag.
   */
  hold(value = this.value) {
    live.delete(this);
    this.settled = true;
    this.value = value;
    this.target = value;
    this.onchange?.(this.value, this);
    return this;
  }

  /** Write a value from a gesture. 1:1, no smoothing -- the finger is truth. */
  set(value) {
    this.value = value;
    this.target = value;
    this.onchange?.(this.value, this);
    return this;
  }

  _step(dt) {
    const omega = (2 * Math.PI) / this.response;
    const zeta = this.damping;
    let x = this.value - this.target;
    let v = this.velocity;

    if (zeta < 1) {
      const omegaD = omega * Math.sqrt(1 - zeta * zeta);
      const decay = Math.exp(-zeta * omega * dt);
      const cosine = Math.cos(omegaD * dt);
      const sine = Math.sin(omegaD * dt);
      const a = x;
      const b = (v + zeta * omega * x) / omegaD;
      const nextX = decay * (a * cosine + b * sine);
      v = decay * (-zeta * omega * (a * cosine + b * sine) + omegaD * (b * cosine - a * sine));
      x = nextX;
    } else {
      const decay = Math.exp(-omega * dt);
      const a = x;
      const b = v + omega * x;
      const nextX = (a + b * dt) * decay;
      v = (b - omega * (a + b * dt)) * decay;
      x = nextX;
    }

    this.velocity = v;
    this.value = this.target + x;

    if (Math.abs(x) < this.epsilon && Math.abs(v) < this.epsilon * omega) {
      this.jump(this.target);
      return;
    }
    this.onchange?.(this.value, this);
  }
}

// --- Momentum ------------------------------------------------------------

/**
 * Where a flick would come to rest, using the same exponential decay as scroll
 * deceleration. Snap to the point nearest *this*, not nearest the release
 * point -- that is the difference between a throw and a drop.
 */
export function project(initialVelocity, decelerationRate = 0.998) {
  return ((initialVelocity / 1000) * decelerationRate) / (1 - decelerationRate);
}

/** The nearest value in `points` to `value`. */
export function nearest(points, value) {
  return points.reduce(
    (best, point) => (Math.abs(point - value) < Math.abs(best - value) ? point : best),
    points[0]
  );
}

/**
 * Progressive resistance past a boundary. A hard stop reads as frozen; this
 * reads as "still listening, but there is nothing more this way".
 */
export function rubberband(overshoot, dimension, constant = 0.55) {
  if (overshoot === 0 || dimension <= 0) return 0;
  return (overshoot * dimension * constant) / (dimension + constant * Math.abs(overshoot));
}

export function clamp(value, low, high) {
  return Math.min(high, Math.max(low, value));
}

// --- Velocity ------------------------------------------------------------

/**
 * Velocity from a short trailing window rather than the last two events. One
 * pair of points is noisy enough to send a gentle release flying.
 */
export function velocityTracker(windowMs = 90) {
  let samples = [];
  return {
    reset(value, time = performance.now()) {
      samples = [{ value, time }];
    },
    push(value, time = performance.now()) {
      samples.push({ value, time });
      while (samples.length > 2 && time - samples[0].time > windowMs) samples.shift();
    },
    /** Units per second. */
    velocity() {
      if (samples.length < 2) return 0;
      const first = samples[0];
      const last = samples[samples.length - 1];
      const dt = (last.time - first.time) / 1000;
      if (dt <= 0) return 0;
      return (last.value - first.value) / dt;
    }
  };
}

/**
 * A gesture is only a drag once it has travelled far enough to mean it. Below
 * the threshold the same pointer sequence is still a tap.
 */
export const DRAG_THRESHOLD = 10;
