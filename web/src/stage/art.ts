// Water above, water below, and the two colours of the veil.
const inks = [
  ['#0f3440', '#2f7f8c', '#f7b79a', '#9fd8e6'], ['#142b4a', '#3f78a8', '#ffc9ad', '#b9e3ef'], ['#12382f', '#3f8f78', '#f4d7a1', '#a9e2c8'],
  ['#3a1f35', '#8a4a66', '#ffc2a8', '#f3dfe6'], ['#1d2347', '#5a62a6', '#f2b5c4', '#b9c8f5'], ['#402318', '#a35b3c', '#ffd9a0', '#f5b598'],
  ['#10303a', '#4f8f86', '#f29f86', '#d8efe2'], ['#2b2440', '#6f5a94', '#f6cfa3', '#c7b8ec'], ['#163436', '#62806a', '#f1d39b', '#cfe6c4'],
];

/** The picture of music that has no cover: a goldfish veil in water, never twice the same. */
export function art(title: string, artist = '') {
  let h = 2166136261;
  for (const c of `${title}/${artist}`) h = Math.imul(h ^ c.codePointAt(0)!, 16777619) >>> 0;
  let seed = h;
  const r = () => { seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0; return seed / 4294967296; };
  const n = (v: number) => v.toFixed(1);
  const [deep, shoal, a, b] = inks[h % inks.length];
  const left = r() < .5;
  // The veil opens from a root near one side, towards the other.
  const x = 256 + (left ? -1 : 1) * (96 + r() * 70), y = 190 + r() * 110;
  const aim = (left ? 0 : Math.PI) + (r() - .5) * .7, fan = 1.5 + r() * .7, count = 5 + Math.floor(r() * 3);
  const at = (ang: number, d: number) => `${n(x + Math.cos(ang) * d)} ${n(y + Math.sin(ang) * d)}`;
  let veil = '', veins = '';
  for (let i = 0; i < count; i++) {
    const t = aim - fan / 2 + fan * i / (count - 1), len = 250 + r() * 130, w = .2 + r() * .16, sway = (r() - .5) * .34;
    const tip = at(t + sway, len);
    veil += `<path d="M${n(x)} ${n(y)}C${at(t - w, len * .34)} ${at(t - w * 1.5, len * .78)} ${tip}C${at(t + w * .5 + sway, len * .86)} ${at(t + w * 1.5, len * .5)} ${n(x)} ${n(y)}Z" fill="url(#${i % 2 ? 'v' : 'u'})" fill-opacity="${n(.42 + r() * .28)}"/>`;
    for (const k of [-.5, 0, .5]) veins += `<path d="M${n(x)} ${n(y)}Q${at(t + w * k * 1.4, len * .5)} ${at(t + sway + w * k * .5, len * .92)}"/>`;
  }
  let pearls = '';
  for (let i = 0; i < 7; i++) {
    const s = 5 + r() * 19;
    pearls += `<circle cx="${n(x + (left ? -1 : 1) * (r() * 130 - 34))}" cy="${n(y - 150 + r() * 270)}" r="${n(s)}" fill="#fff" fill-opacity="${n(.06 + r() * .1)}" stroke="#fff" stroke-opacity=".5" stroke-width="1.4"/>`;
  }
  let swell = '';
  for (let i = 0; i < 3; i++) {
    const yy = 372 + i * 48, d = 16 + r() * 22, o = r() * 60;
    swell += `<path d="M-20 ${n(yy)}C${n(110 + o)} ${n(yy - d)} ${n(170 + o)} ${n(yy + d)} 290 ${n(yy)}S${n(430 + o)} ${n(yy - d)} 540 ${n(yy + d * .4)}V520H-20Z" fill="${deep}" fill-opacity="${n(.2 + i * .13)}"/>`;
  }
  const grad = (id: string, p: string, q: string) => `<radialGradient id="${id}" gradientUnits="userSpaceOnUse" cx="${n(x)}" cy="${n(y)}" r="380"><stop stop-color="#fff" stop-opacity=".72"/><stop offset=".34" stop-color="${p}"/><stop offset="1" stop-color="${q}"/></radialGradient>`;
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512"><defs><linearGradient id="w" x1="${left ? 0 : 1}" y1="0" x2="${left ? 1 : 0}" y2="1"><stop stop-color="${shoal}"/><stop offset="1" stop-color="${deep}"/></linearGradient>${grad('u', b, a)}${grad('v', a, b)}</defs><rect width="512" height="512" fill="url(#w)"/>${swell}${veil}<g fill="none" stroke="#fff" stroke-opacity=".2" stroke-width="1.3">${veins}</g>${pearls}</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

/** Cover files come from 飞牛音乐 through our own server. */
export function coverURL(id: string | undefined, size: 160 | 600 | 1024 = 600) {
  return id && /^[\w-]+$/.test(id) ? `/api/cover/${id}?size=${size}` : '';
}
