// A QR code of version 5, level L: 37 modules, up to 106 bytes. Enough for a login address.
const SIZE = 37, DATA = 108, EC = 26;

const exp: number[] = [], log: number[] = [];
for (let i = 0, x = 1; i < 255; i++) { exp[i] = x; log[x] = i; x <<= 1; if (x & 256) x ^= 0x11d; }
const mul = (a: number, b: number) => (a && b ? exp[(log[a] + log[b]) % 255] : 0);

function correction(data: number[]) {
  let gen = [1];
  for (let i = 0; i < EC; i++) {
    const next = Array<number>(gen.length + 1).fill(0);
    gen.forEach((c, j) => { next[j] ^= c; next[j + 1] ^= mul(c, exp[i]); });
    gen = next;
  }
  const rest = [...data, ...Array<number>(EC).fill(0)];
  for (let i = 0; i < data.length; i++) { const k = rest[i]; if (k) gen.forEach((c, j) => { rest[i + j] ^= mul(c, k); }); }
  return rest.slice(data.length);
}

/** Rows of dark (true) and light modules. */
export function qr(text: string): boolean[][] {
  const bytes = [...new TextEncoder().encode(text)];
  if (bytes.length > 106) throw new Error('too long');
  const bits: number[] = [];
  const put = (v: number, n: number) => { for (let i = n - 1; i >= 0; i--) bits.push((v >> i) & 1); };
  put(4, 4); put(bytes.length, 8); bytes.forEach((b) => put(b, 8)); put(0, Math.min(4, DATA * 8 - bits.length));
  while (bits.length % 8) bits.push(0);
  const data: number[] = [];
  for (let i = 0; i < bits.length; i += 8) data.push(parseInt(bits.slice(i, i + 8).join(''), 2));
  for (let pad = 0xec; data.length < DATA; pad ^= 0xec ^ 0x11) data.push(pad);
  const all = [...data, ...correction(data)].flatMap((b) => [7, 6, 5, 4, 3, 2, 1, 0].map((i) => (b >> i) & 1));

  const m = Array.from({ length: SIZE }, () => Array<boolean | null>(SIZE).fill(null));
  const block = (cy: number, cx: number, r: number, ring: number) => {
    for (let y = -r; y <= r; y++) for (let x = -r; x <= r; x++) {
      const d = Math.max(Math.abs(x), Math.abs(y));
      if (m[cy + y]?.[cx + x] !== undefined) m[cy + y][cx + x] = d !== ring && d !== 4;
    }
  };
  block(3, 3, 4, 2); block(3, SIZE - 4, 4, 2); block(SIZE - 4, 3, 4, 2); block(30, 30, 2, 1);
  for (let i = 8; i < SIZE - 8; i++) { m[6][i] = i % 2 === 0; m[i][6] = i % 2 === 0; }
  // Level L, mask 0, with its check bits.
  const fmt = 0b111011111000100, f = (i: number) => ((fmt >> i) & 1) === 1;
  for (let i = 0; i < 15; i++) {
    m[i < 6 ? i : i < 8 ? i + 1 : SIZE - 15 + i][8] = f(i);
    m[8][i < 8 ? SIZE - 1 - i : i < 9 ? 7 : 14 - i] = f(i);
  }
  m[SIZE - 8][8] = true;
  let n = 0;
  for (let right = SIZE - 1, up = true; right > 0; right -= 2, up = !up) {
    if (right === 6) right--;
    for (let i = 0; i < SIZE; i++) {
      const y = up ? SIZE - 1 - i : i;
      for (const x of [right, right - 1]) if (m[y][x] === null) m[y][x] = ((all[n++] ?? 0) ^ ((x + y) % 2 === 0 ? 1 : 0)) === 1;
    }
  }
  return m as boolean[][];
}
