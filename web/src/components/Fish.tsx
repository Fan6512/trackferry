import { useEffect, useRef } from 'react';
import atlasURL from '../assets/brand/fish-atlas.webp';
import './fish.css';

export type FishMood = 'idle' | 'searching' | 'sending' | 'waiting' | 'confirmed' | 'uncertain';
export type FishFrame = 'idle' | 'ready' | 'dart' | 'wink';

const cells: Record<FishFrame, [number, number]> = { idle: [0, 0], ready: [1, 0], dart: [0, 1], wink: [1, 1] };
const CELL = 512, STRIPS = 64;
let atlas: HTMLImageElement | null = null;

/** The painted frames; null until they have loaded. */
export function fishAtlas() {
  if (!atlas) { atlas = new Image(); atlas.src = atlasURL; }
  return atlas.complete && atlas.naturalWidth ? atlas : null;
}

/** Paints one frame into a square, its veil moving like water: still at the head, wide at the tail. */
function paint(g: CanvasRenderingContext2D, size: number, frame: FishFrame, phase: number, amp: number) {
  const img = fishAtlas();
  if (!img) return;
  const [cx, cy] = cells[frame];
  const n = Math.max(20, Math.min(STRIPS, size >> 3));
  for (let i = 0; i < n; i++) {
    const a = Math.round(i * size / n), b = Math.round((i + 1) * size / n), back = 1 - i / (n - 1);
    const y = amp * size * back ** 1.7 * Math.sin(phase - back * 3.6);
    g.drawImage(img, cx * CELL + a / size * CELL, cy * CELL, (b - a) / size * CELL, CELL, a, y, b - a, size);
  }
}

/** A fish that never jumps: its beat is carried on from frame to frame, its looks fade into each other. */
export class Swimmer {
  readonly canvas = document.createElement('canvas');
  private g = this.canvas.getContext('2d')!;
  private other: HTMLCanvasElement | null = null;
  private phase = 0;
  private amp = .02;
  private speed = 1.4;
  private frame: FishFrame = 'idle';
  private was: FishFrame = 'idle';
  private mix = 1;

  paint(size: number, dt: number, frame: FishFrame, amp: number, speed: number) {
    const k = 1 - Math.exp(-dt * 2.6);
    this.amp += (amp - this.amp) * k;
    this.speed += (speed - this.speed) * k;
    this.phase = (this.phase + dt * this.speed) % (Math.PI * 2);
    if (frame !== this.frame && this.mix === 1) { this.was = this.frame; this.frame = frame; this.mix = 0; }
    this.mix = Math.min(1, this.mix + dt / .32);
    const c = this.canvas, g = this.g;
    if (c.width !== size) c.width = c.height = size;
    g.globalCompositeOperation = 'source-over';
    g.globalAlpha = 1;
    g.clearRect(0, 0, size, size);
    paint(g, size, this.frame, this.phase, this.amp);
    if (this.mix === 1) return c;
    const o = this.other ??= document.createElement('canvas');
    if (o.width !== size) o.width = o.height = size;
    const h = o.getContext('2d')!;
    h.clearRect(0, 0, size, size);
    paint(h, size, this.was, this.phase, this.amp);
    const m = this.mix * this.mix * (3 - 2 * this.mix);
    // Keep m of the new look, add the rest of the old one.
    g.globalCompositeOperation = 'destination-in';
    g.fillStyle = `rgba(0,0,0,${m})`;
    g.fillRect(0, 0, size, size);
    g.globalCompositeOperation = 'lighter';
    g.globalAlpha = 1 - m;
    g.drawImage(o, 0, 0);
    return c;
  }
}

interface Pose { frame: FishFrame; x: number; y: number; turn: number; face: number; amp: number; speed: number; alpha: number }

function pose(mood: FishMood, age: number, t: number): Pose {
  const p: Pose = { frame: 'idle', x: 0, y: Math.sin(t * .8) * .018, turn: Math.sin(t * .55) * .03, face: 1, amp: .026, speed: 1.5, alpha: 1 };
  switch (mood) {
    case 'searching': return { ...p, x: Math.sin(t * .85) * .06, face: Math.tanh(Math.cos(t * .85) * 2.6), amp: .034, speed: 2.3 };
    case 'sending':
      if (age < .4) return { ...p, frame: 'ready', x: -.04, amp: .012 };
      if (age < 1.5) return { ...p, frame: 'dart', x: .13, y: p.y - .04, turn: -.1, amp: .04, speed: 4 };
      return p;
    case 'waiting': return { ...p, frame: 'ready', amp: .014, speed: 1.1, turn: p.turn * .4 };
    case 'confirmed':
      if (age < .5) return { ...p, y: p.y - .05, turn: -.1, amp: .036, speed: 3 };
      if (age < 2) return { ...p, frame: 'wink' };
      return p;
    case 'uncertain': return { ...p, turn: .1, amp: .012, speed: .8, alpha: .7 };
    default: return t % 11 > 10.3 ? { ...p, frame: 'wink' } : p;
  }
}

/** The goldfish. `water`: its bottom edge is a waterline and it is mirrored below. */
export function Fish({ mood = 'idle', water = false, className = '' }: { mood?: FishMood; water?: boolean; className?: string }) {
  const cv = useRef<HTMLCanvasElement>(null);
  const now = useRef({ mood, since: 0, poke: -9 });
  useEffect(() => { now.current.mood = mood; now.current.since = performance.now() / 1000; }, [mood]);

  useEffect(() => {
    const c = cv.current!, g = c.getContext('2d');
    if (!g) return;
    const still = matchMedia('(prefers-reduced-motion: reduce)').matches;
    const swim = new Swimmer();
    const at = { x: 0, y: 0, turn: 0, alpha: 1 };
    let raf = 0, last = 0, seen = true, size = 0;
    const born = performance.now() / 1000;
    const bubbles: { x: number; y: number; r: number; at: number }[] = [];
    let blow = born + 2;

    const draw = (wall: number) => {
      raf = (still && fishAtlas()) || !seen ? 0 : requestAnimationFrame(draw);
      // A small fish does not need every frame of the display.
      if (!size || (!still && wall - last < (size < 200 ? 30 : 14))) return;
      const dt = Math.min(.05, (wall - last) / 1000);
      last = wall;
      const t = still ? 0 : wall / 1000;
      const { mood: m, since, poke } = now.current;
      const poked = t - poke < 2 && !still;
      const p = pose(poked ? 'confirmed' : m, poked ? t - poke : still ? 99 : t - since, t);
      const body = swim.paint(size, still ? 0 : dt, p.frame, p.amp, p.speed);
      const k = still ? 1 : 1 - Math.exp(-dt * 4.5);
      for (const n of ['x', 'y', 'turn', 'alpha'] as const) at[n] += (p[n] - at[n]) * k;
      const arrive = still ? 1 : 1 - Math.exp(-(t - born) * 2.2);
      g.clearRect(0, 0, c.width, c.height);
      const lift = water ? size * .13 : 0;
      const put = (mirror: boolean) => {
        g.save();
        const y = size / 2 + lift + at.y * size;
        g.translate(size / 2 + at.x * size - (1 - arrive) * size * .35, mirror ? 2 * size - y : y);
        g.scale(p.face, mirror ? -1 : 1);
        g.rotate(at.turn);
        g.globalAlpha = at.alpha * arrive * (mirror ? .2 : 1);
        g.drawImage(body, -size / 2, -size / 2);
        g.restore();
      };
      if (water) put(true);
      put(false);
      if (!still) {
        if (t > blow) {
          bubbles.push({ x: size * (.5 + at.x + .37 * p.face), y: lift + size * (.4 + at.y), r: size * (.008 + Math.random() * .012), at: t });
          blow = t + (m === 'searching' ? 1.6 : 4.5) + Math.random() * 2.5;
        }
        for (let i = bubbles.length - 1; i >= 0; i--) {
          const q = bubbles[i], a = (t - q.at) / 3.6;
          if (a > 1) { bubbles.splice(i, 1); continue; }
          g.beginPath();
          g.arc(q.x + Math.sin(a * 7 + q.r) * size * .012, q.y - a * size * .34, q.r * (1 + a * .5), 0, 7);
          g.lineWidth = Math.max(1, size * .003);
          g.strokeStyle = `rgba(226,244,246,${.75 * (1 - a)})`;
          g.stroke();
        }
      }
    };
    const fit = new ResizeObserver(() => {
      const w = c.clientWidth;
      size = Math.min(960, Math.round(w * Math.min(devicePixelRatio || 1, 2)));
      c.width = size;
      c.height = Math.round(size * (water ? 1.5 : 1));
      if (still && !raf) raf = requestAnimationFrame(draw);
    });
    fit.observe(c);
    const see = new IntersectionObserver(([e]) => { seen = e.isIntersecting; if (seen && !raf) raf = requestAnimationFrame(draw); });
    see.observe(c);
    raf = requestAnimationFrame(draw);
    return () => { cancelAnimationFrame(raf); fit.disconnect(); see.disconnect(); };
  }, [water]);

  return <canvas ref={cv} className={`fish ${water ? 'fish-water' : ''} ${className}`} aria-hidden="true" onPointerDown={() => { now.current.poke = performance.now() / 1000; }} />;
}
