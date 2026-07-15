// A synthetic pointer overlay for automated demos and screen recordings.
// Automation dispatches picks as synthetic DOM events, so the real OS cursor
// never moves — a recording would show the model and editor reacting with no
// visible cause. DemoCursor draws a pointer that travels to each target and
// pulses on click, making the interaction legible on video.
//
// Coordinates are viewport (client) coordinates, matching the clientX/clientY a
// caller derives from a canvas rect, so the pointer tip lands exactly where the
// pick is dispatched. The element is fixed-positioned and never intercepts
// input (pointer-events: none).

// Arrow with its tip near the SVG origin (2,2); the element is translated by
// (x - TIP, y - TIP) so the tip sits on the target point.
const TIP = 2;
const POINTER_SVG = `<svg width="26" height="30" viewBox="0 0 24 28" xmlns="http://www.w3.org/2000/svg">
  <path d="M2 2 L2 23 L7.4 18 L11 25.5 L14.2 24 L10.6 16.7 L18 16.7 Z"
    fill="#ffffff" stroke="#111111" stroke-width="1.6" stroke-linejoin="round"/>
</svg>`;

const TRAVEL_EASE = 'cubic-bezier(0.22, 1, 0.36, 1)';

// Resolve when the animation finishes OR after its duration elapses — WKWebView
// pauses rAF (and thus animation.finished) when the window isn't frontmost, which
// would otherwise hang an automation command awaiting the pointer. The timeout is
// the safety net; normally .finished wins.
function settle(anim: Animation, ms: number): Promise<void> {
  return Promise.race([
    anim.finished.then(() => undefined, () => undefined),
    new Promise<void>((r) => setTimeout(r, ms + 100)),
  ]);
}

export class DemoCursor {
  private el: HTMLDivElement | null = null;
  private x = 0;
  private y = 0;
  private placed = false;

  private ensure(): HTMLDivElement {
    if (this.el) return this.el;
    const el = document.createElement('div');
    el.style.cssText = [
      'position:fixed',
      'left:0',
      'top:0',
      'z-index:2147483647',
      'pointer-events:none',
      'opacity:0',
      'will-change:transform',
      'filter:drop-shadow(0 2px 3px rgba(0,0,0,0.45))',
    ].join(';');
    el.innerHTML = POINTER_SVG;
    document.body.appendChild(el);
    this.el = el;
    return el;
  }

  private restingTransform(): string {
    return `translate(${this.x - TIP}px, ${this.y - TIP}px)`;
  }

  /** Move the pointer to a viewport point over `ms`, resolving when it arrives.
   *  The first move pops the pointer in at the target; later moves travel. */
  async moveTo(x: number, y: number, ms = 520): Promise<void> {
    const el = this.ensure();
    if (!this.placed) {
      this.x = x;
      this.y = y;
      this.placed = true;
      el.style.opacity = '1';
      el.style.transform = this.restingTransform();
      await settle(el.animate([{ opacity: 0 }, { opacity: 1 }], { duration: 200 }), 200);
      return;
    }
    const from = this.restingTransform();
    this.x = x;
    this.y = y;
    const to = this.restingTransform();
    el.style.transform = to; // inline holds the resting state; anim has no fill
    await settle(el.animate(
      [{ transform: from }, { transform: to }],
      { duration: ms, easing: TRAVEL_EASE },
    ), ms);
  }

  /** Play a click: a ripple ring plus a brief press dip on the pointer. */
  async click(): Promise<void> {
    const el = this.ensure();
    this.ripple();
    const rest = this.restingTransform();
    await settle(el.animate(
      [
        { transform: `${rest} scale(1)` },
        { transform: `${rest} scale(0.82)`, offset: 0.4 },
        { transform: `${rest} scale(1)` },
      ],
      { duration: 260, easing: 'ease-out' },
    ), 260);
  }

  private ripple(): void {
    const size = 16;
    const r = document.createElement('div');
    r.style.cssText = [
      'position:fixed',
      `left:${this.x - size / 2}px`,
      `top:${this.y - size / 2}px`,
      `width:${size}px`,
      `height:${size}px`,
      'border-radius:50%',
      'border:2px solid rgba(255,255,255,0.95)',
      'box-shadow:0 0 0 1px rgba(0,0,0,0.35)',
      'z-index:2147483646',
      'pointer-events:none',
    ].join(';');
    document.body.appendChild(r);
    r.animate(
      [
        { transform: 'scale(0.4)', opacity: 0.9 },
        { transform: 'scale(3.4)', opacity: 0 },
      ],
      { duration: 520, easing: 'ease-out' },
    ).finished.then(() => r.remove());
  }

  /** Fade the pointer out and remove it. */
  async hide(): Promise<void> {
    if (!this.el) return;
    const el = this.el;
    this.el = null;
    this.placed = false;
    await settle(el.animate([{ opacity: 1 }, { opacity: 0 }], { duration: 250 }), 250);
    el.remove();
  }
}
