// Toggle — a boolean state whose changes fan out to every affordance that
// reflects or flips it (toolbar button, menu item, automation command), so they
// can never desync. Replaces the ad-hoc "set the button's active class inside
// the click handler" wiring, which silently drifts the moment the same state is
// changed from somewhere else (a menu action, or automation driving it).
//
// The owner supplies `apply`, which performs the real effect (e.g.
// viewer.setAutoRotate); the Toggle runs it on every change and then notifies
// subscribers. Bind a button with bindToggleButton and drive the same instance
// from menus/automation — one source of truth.

export class Toggle {
  private state: boolean;
  private readonly listeners = new Set<(on: boolean) => void>();

  constructor(initial: boolean, private readonly apply: (on: boolean) => void) {
    this.state = initial;
  }

  get(): boolean {
    return this.state;
  }

  /** Set the state. No-op (and no apply/notify) when already in that state. */
  set(on: boolean): void {
    if (this.state === on) return;
    this.state = on;
    this.apply(on);
    for (const listener of this.listeners) listener(on);
  }

  toggle(): boolean {
    this.set(!this.state);
    return this.state;
  }

  /** Subscribe to changes. Fires immediately with the current state so the
   *  affordance starts in sync. Returns an unsubscribe function. */
  subscribe(listener: (on: boolean) => void): () => void {
    this.listeners.add(listener);
    listener(this.state);
    return () => this.listeners.delete(listener);
  }
}

/** Wire a toolbar button to a Toggle: a click flips it, and the button's
 *  `active` class follows the toggle no matter what changes it. */
export function bindToggleButton(btn: HTMLElement, toggle: Toggle): void {
  btn.addEventListener('click', () => toggle.toggle());
  toggle.subscribe((on) => btn.classList.toggle('active', on));
}
