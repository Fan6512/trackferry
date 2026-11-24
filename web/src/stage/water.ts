import { frag, vert } from './shader';

export interface Rect { x: number; y: number; w: number; h: number }
export interface Picture { src: string; fallback: string; cutout: boolean }

const SIZE = 512;
const ease = (from: number, to: number, dt: number, rate: number) => to + (from - to) * Math.exp(-dt * rate);

/** Draws the sky and the water the whole app floats on. */
export class Water {
  private gl: WebGL2RenderingContext | null;
  private u: Record<string, WebGLUniformLocation | null> = {};
  private tex: WebGLTexture[] = [];
  private cut = [0, 0];
  private mix = 1;
  private obj = 0;
  private objTo = 0;
  private rect: Rect = { x: 0, y: 0, w: 1, h: 1 };
  private rectTo: Rect | null = null;
  private line = 0;
  private lineTo = 0;
  private placed = false;
  private rip = new Float32Array(18).fill(-99);
  private ripAt = 0;
  private stone = { x: 0, on: 0 };
  private path: number[] = [];
  private hold = 0;
  private want: Picture | null = null;
  private loading = '';
  private shown = '';
  private t = 0;
  private last = 0;
  private raf = 0;
  private dpr = 1;
  private slow = 0;
  private scale = 1;
  readonly still = matchMedia('(prefers-reduced-motion: reduce)').matches;

  constructor(private cv: HTMLCanvasElement) {
    this.gl = cv.getContext('webgl2', { antialias: false, alpha: false, powerPreference: 'low-power' });
    if (!this.gl) return;
    this.init(this.gl);
    cv.addEventListener('webglcontextlost', (e) => { e.preventDefault(); cancelAnimationFrame(this.raf); });
    cv.addEventListener('webglcontextrestored', () => { this.tex = []; this.shown = ''; this.init(this.gl!); this.next(); this.wake(); });
    addEventListener('resize', this.resize);
    document.addEventListener('visibilitychange', this.wake);
    this.resize();
    this.line = this.lineTo = innerHeight * 0.64;
    this.draw();
  }

  get ok() { return !!this.gl; }

  dispose() {
    cancelAnimationFrame(this.raf);
    removeEventListener('resize', this.resize);
    document.removeEventListener('visibilitychange', this.wake);
  }

  private init(gl: WebGL2RenderingContext) {
    const mk = (type: number, src: string) => {
      const s = gl.createShader(type)!;
      gl.shaderSource(s, src);
      gl.compileShader(s);
      if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) throw new Error(gl.getShaderInfoLog(s) ?? 'shader');
      return s;
    };
    const pr = gl.createProgram()!;
    try {
      gl.attachShader(pr, mk(gl.VERTEX_SHADER, vert));
      gl.attachShader(pr, mk(gl.FRAGMENT_SHADER, frag));
      gl.linkProgram(pr);
    } catch (e) {
      console.error(e);
      this.gl = null;
      return;
    }
    gl.useProgram(pr);
    gl.bindBuffer(gl.ARRAY_BUFFER, gl.createBuffer());
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
    const loc = gl.getAttribLocation(pr, 'p');
    gl.enableVertexAttribArray(loc);
    gl.vertexAttribPointer(loc, 2, gl.FLOAT, false, 0, 0);
    for (const n of ['uRes', 'uDpr', 'uT', 'uA', 'uB', 'uMix', 'uObj', 'uRect', 'uH', 'uCut', 'uCutA', 'uStone', 'uStoneOn', 'uRip']) this.u[n] = gl.getUniformLocation(pr, n);
    for (let i = 0; i < 2; i++) {
      const t = gl.createTexture()!;
      gl.bindTexture(gl.TEXTURE_2D, t);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, new Uint8Array([34, 58, 70, 255]));
      gl.generateMipmap(gl.TEXTURE_2D);
      for (const [k, v] of [[gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR], [gl.TEXTURE_MAG_FILTER, gl.LINEAR], [gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE], [gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE]] as const) gl.texParameteri(gl.TEXTURE_2D, k, v);
      this.tex.push(t);
    }
  }

  /** The picture whose colours fill the sky, and which floats when a place is set. */
  show(p: Picture) {
    if (this.want?.src === p.src) return;
    this.want = p;
    this.next();
  }

  private next() {
    const p = this.want;
    if (!p || !this.gl || this.loading || this.mix < 1 || p.src === this.shown) return;
    this.loading = p.src;
    const done = (img: HTMLImageElement | null) => {
      this.loading = '';
      if (img && this.gl) {
        this.upload(img, p.cutout);
        this.shown = p.src;
      } else this.shown = p.src;
      if (this.want !== p) this.next();
    };
    const img = new Image();
    img.onload = () => done(img);
    img.onerror = () => {
      const alt = new Image();
      alt.onload = () => done(alt);
      alt.onerror = () => done(null);
      alt.src = p.fallback;
    };
    img.src = p.src;
  }

  private upload(img: HTMLImageElement, cutout: boolean) {
    const gl = this.gl!;
    const c = document.createElement('canvas');
    c.width = c.height = SIZE;
    const x = c.getContext('2d')!;
    // A cut-out still needs colours around it for the sky.
    const s = Math.min(img.naturalWidth, img.naturalHeight) || SIZE;
    x.imageSmoothingQuality = 'high';
    x.drawImage(img, (img.naturalWidth - s) / 2, (img.naturalHeight - s) / 2, s, s, 0, 0, SIZE, SIZE);
    const data = x.getImageData(0, 0, SIZE, SIZE);
    if (cutout) {
      const d = data.data;
      for (let i = 0; i < d.length; i += 4) if (d[i + 3] < 8) {
        const y = i / 4 / SIZE / SIZE;
        d[i] = 22 + y * 60; d[i + 1] = 62 + y * 30; d[i + 2] = 82 - y * 14;
      }
    }
    this.tex.reverse();
    this.cut = [this.cut[1], cutout ? 1 : 0];
    gl.bindTexture(gl.TEXTURE_2D, this.tex[1]);
    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, data);
    gl.generateMipmap(gl.TEXTURE_2D);
    this.mix = this.still || this.shown === '' ? 1 : 0;
    if (this.shown === '' && !this.still && this.objTo > 0) this.mix = 0.46;
    this.wake();
  }

  /** Where the picture floats (null: only its colours stay) and where the water begins. */
  place(rect: Rect | null, line: number) {
    const first = !this.placed;
    const sharp = !!rect !== !!this.objTo;
    this.placed = true;
    this.objTo = rect ? 1 : 0;
    this.rectTo = rect;
    this.lineTo = line;
    if (rect && (first || this.obj < 0.02)) this.rect = { ...rect };
    if (first || this.still) this.line = line;
    if (this.still) this.obj = this.objTo;
    if (sharp) this.resize(); else this.wake();
  }

  /** Follows layout exactly while scrolling. */
  nudge(rect: Rect | null, line: number) {
    if (rect) this.rect = { ...rect };
    this.rectTo = rect;
    this.line = this.lineTo = line;
    this.wake();
  }

  ripple(x: number, y: number) {
    if (this.still) return;
    this.rip.set([x, y, this.t], this.ripAt * 3);
    this.ripAt = (this.ripAt + 1) % 6;
  }

  /** Sends the stone along these points of the waterline; empty hides it. */
  ferry(xs: number[], instant = false) {
    this.path = [...xs];
    this.hold = 0;
    if (!xs.length) return;
    if (instant || this.still) {
      this.stone.x = xs[xs.length - 1];
      this.path = [this.stone.x];
    } else this.stone.x = xs[0] - 90;
    this.wake();
  }

  get waterline() { return this.line; }

  private resize = () => {
    if (!this.gl) return;
    this.dpr = Math.min(devicePixelRatio || 1, 1.5) * this.scale * (this.objTo ? 1 : .72); // colours alone need no detail
    this.cv.width = Math.round(innerWidth * this.dpr);
    this.cv.height = Math.round(innerHeight * this.dpr);
    this.gl.viewport(0, 0, this.cv.width, this.cv.height);
    this.wake();
  };

  private wake = () => {
    cancelAnimationFrame(this.raf);
    if (document.hidden || !this.gl) return;
    this.last = performance.now() - 100; // draw on the very next frame
    this.raf = requestAnimationFrame(this.frame);
  };

  private frame = (now: number) => {
    const ms = now - this.last;
    // The water is calm: half the display's frames are enough unless something moves.
    const busy = this.mix < 1 || this.path.length > 1 || Math.abs(this.obj - this.objTo) > 0.01 || Math.abs(this.line - this.lineTo) > 0.5;
    if (ms < (busy ? 12 : this.objTo ? 30 : 40)) {
      this.raf = requestAnimationFrame(this.frame);
      return;
    }
    const dt = Math.min(ms, 250) / 1000; // slow frames must not stretch the transitions
    this.last = now;
    this.t += this.still ? 0 : dt;
    // A slow machine gets fewer pixels, not fewer frames.
    if (busy) this.slow = ms > 40 ? this.slow + 1 : Math.max(0, this.slow - 1);
    if (this.slow > 24 && this.scale > 0.5) { this.scale *= 0.75; this.slow = 0; this.resize(); }
    this.step(dt);
    this.draw();
    cancelAnimationFrame(this.raf); // a resize above may have asked already
    if (!this.still) this.raf = requestAnimationFrame(this.frame);
  };

  private step(dt: number) {
    if (this.mix < 1) {
      const was = this.mix;
      this.mix = Math.min(1, this.mix + dt / 1.7);
      const b = this.rect;
      if (this.obj > 0.5) {
        if (was < 0.05 && this.mix >= 0.05) this.ripple(b.x + b.w / 2, this.line + 8);
        if (was < 0.55 && this.mix >= 0.55) this.ripple(b.x + b.w / 2, this.line + 8);
      }
      if (this.mix === 1) this.next();
    }
    this.obj = ease(this.obj, this.objTo, dt, 5);
    this.line = ease(this.line, this.lineTo, dt, 5);
    if (this.rectTo) for (const k of ['x', 'y', 'w', 'h'] as const) this.rect[k] = ease(this.rect[k], this.rectTo[k], dt, 6);
    const on = this.path.length ? 1 : 0;
    this.stone.on = ease(this.stone.on, on, dt, 4);
    if (this.path.length) {
      const to = this.path[0];
      if (this.hold > 0) this.hold -= dt;
      else this.stone.x = ease(this.stone.x, to, dt, 6.5);
      if (Math.abs(this.stone.x - to) < 1.5 && this.path.length > 1 && this.hold <= 0) {
        this.ripple(to, this.line + 4);
        this.path.shift();
        this.hold = 0.16;
      }
    }
  }

  private draw() {
    const gl = this.gl!;
    const u = this.u;
    for (let i = 0; i < 2; i++) { gl.activeTexture(gl.TEXTURE0 + i); gl.bindTexture(gl.TEXTURE_2D, this.tex[i]); }
    gl.uniform1i(u.uA, 0);
    gl.uniform1i(u.uB, 1);
    gl.uniform2f(u.uRes, innerWidth, innerHeight);
    gl.uniform1f(u.uDpr, this.dpr);
    gl.uniform1f(u.uT, this.t);
    gl.uniform1f(u.uMix, this.mix);
    gl.uniform1f(u.uObj, this.obj);
    gl.uniform4f(u.uRect, this.rect.x, this.rect.y, this.rect.w, this.rect.h);
    gl.uniform1f(u.uH, this.line);
    gl.uniform1f(u.uCutA, this.cut[0]);
    gl.uniform1f(u.uCut, this.cut[1]);
    const bob = Math.sin(this.t * 1.4) * 1.5;
    gl.uniform2f(u.uStone, this.stone.x, this.line - 9 + bob);
    gl.uniform1f(u.uStoneOn, this.stone.on);
    gl.uniform3fv(u.uRip, this.rip);
    gl.drawArrays(gl.TRIANGLES, 0, 3);
  }
}
