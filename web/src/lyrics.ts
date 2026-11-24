export interface Line { at: number; text: string }

/** Reads LRC; lines without a time are dropped. */
export function parseLRC(s: string): Line[] {
  const out: Line[] = [];
  let shift = 0;
  for (const raw of s.split(/\r?\n/)) {
    const o = /^\[offset:\s*([+-]?\d+)\]/i.exec(raw);
    if (o) { shift = Number(o[1]); continue; }
    const times = [...raw.matchAll(/\[(\d+):(\d+(?:[.:]\d+)?)\]/g)];
    const text = raw.replace(/\[[^\]]*\]/g, '').trim();
    if (!text) continue;
    for (const t of times) out.push({ at: Number(t[1]) * 60000 + Number(t[2].replace(':', '.')) * 1000 - shift, text });
  }
  return out.sort((a, b) => a.at - b.at);
}

export function lineAt(lines: Line[], at: number) {
  let i = -1;
  while (i + 1 < lines.length && lines[i + 1].at <= at) i++;
  return i;
}

/** Words without times, as written. */
export function plainLyrics(s: string): Line[] {
  return s.split(/\r?\n/).map((l) => l.replace(/\[[^\]]*\]/g, '').trim()).filter(Boolean).map((text) => ({ at: 0, text }));
}
