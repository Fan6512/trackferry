const still = matchMedia('(prefers-reduced-motion: reduce)').matches;

/** One value on a spring. */
class Spring {
  v = 0;
  to: number;
  constructor(public x: number, private k: number, private c: number) { this.to = x; }
  step(dt: number) { this.v += (-this.k * (this.x - this.to) - this.c * this.v) * dt; this.x += this.v * dt; }
  set(x: number) { this.x = this.to = x; this.v = 0; }
  get calm() { return Math.abs(this.x - this.to) < .002 && Math.abs(this.v) < .02; }
}

/** A spring of this stiffness; ring under 1 overshoots, the less the longer. */
const spring = (k: number, ring: number, x = 0) => new Spring(x, k, 2 * Math.sqrt(k) * ring);

/** The drop that marks the chosen one. It rides almost straight to its place; what wobbles is its
    height, which gives way as it speeds up and rings after it lands. Only transforms change. */
export class Jelly {
  private x = spring(290, .86);
  private tall = spring(300, .22, 1);
  private lift = spring(300, .8);
  private px = spring(320, .6); // the finger beyond the box
  private py = spring(260, .34);
  private all = [this.x, this.tall, this.lift, this.px, this.py];
  private raf = 0;
  private last = 0;
  private placed = false;
  private faces: HTMLElement[];
  held = false;
  constructor(private box: HTMLElement, private lens: HTMLElement, private items: () => HTMLElement[]) { this.faces = [...lens.children] as HTMLElement[]; }

  /** To this place; at once the first time. */
  go(left: number, width: number) {
    if (!this.placed || still) { this.put(left, width); return; }
    if (Math.abs(left - this.x.to) < .5) return;
    this.x.to = left;
    this.wake();
  }
  /** Snaps without a ride, as when the box itself is resized. */
  put(left: number, width: number) { this.placed = true; this.lens.style.width = `${width}px`; this.x.set(left); this.draw(); }
  hold(on: boolean) { this.held = on; if (!on) this.px.to = this.py.to = 0; this.wake(); }
  drag(dx: number, dy: number) {
    const soft = (v: number, most: number) => most * Math.tanh(v / (most * 9));
    this.px.to = soft(dx, 7); this.py.to = soft(dy, 9);
    this.wake();
  }
  stop() { cancelAnimationFrame(this.raf); }

  private wake() { if (!this.raf && !still) { this.last = performance.now(); this.raf = requestAnimationFrame(this.frame); } }
  private frame = (now: number) => {
    let dt = Math.min(.034, (now - this.last) / 1000);
    this.last = now;
    this.lift.to = this.held || Math.abs(this.x.x - this.x.to) > 10 ? 1 : 0;
    for (; dt > 0; dt -= .004) {
      this.tall.to = 1 + .17 * this.lift.to - this.speed * 1.1;
      for (const s of this.all) s.step(Math.min(dt, .004));
    }
    const calm = !this.held && this.all.every((s) => s.calm);
    if (calm) for (const s of this.all) s.set(s.to);
    this.draw();
    this.raf = calm ? 0 : requestAnimationFrame(this.frame);
  };
  /** 0 at rest, .1 at most. */
  private get speed() { return Math.min(.1, Math.abs(this.x.v) / 11000); }
  private draw() {
    const l = Math.max(0, Math.min(1, this.lift.x)), y = this.py.x;
    this.lens.style.transform = `translate3d(${this.x.x}px,${y * .55}px,0) scale(${1 + .05 * l + this.speed},${this.tall.x + Math.abs(y) * .008})`;
    this.faces[0].style.opacity = String(1 - l);
    this.faces[1].style.opacity = String(l);
    for (const el of this.items()) {
      const near = Math.max(0, 1 - Math.abs(el.offsetLeft - this.x.x) / (el.offsetWidth * .9)) * l;
      const pic = el.firstElementChild as HTMLElement | null;
      if (pic?.tagName === 'svg') pic.style.transform = near ? `translateY(${y * .5}px) scale(${1 + .13 * near},${1 + .13 * near + (this.tall.x - 1) * .5 * near})` : '';
    }
    const p = this.px.x, grow = 1 + .014 * l;
    this.box.style.transform = l > .001 || p || y ? `translate3d(${p}px,${y * .4}px,0) scale(${grow + Math.abs(p) * .003 - Math.abs(y) * .002},${grow - Math.abs(p) * .004 + Math.abs(y) * .006})` : '';
  }
}
