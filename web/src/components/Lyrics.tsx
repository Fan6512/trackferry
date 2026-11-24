import { useEffect, useRef, useState } from 'react';
import { api } from '../api';
import { length } from '../copy';
import { position, type Sounding } from '../live';
import { lineAt, parseLRC, plainLyrics, type Line } from '../lyrics';
import { Fish } from './Fish';

export interface Words { lines: Line[]; timed: boolean }

/** The words of a library song; null until they are known, or when there are none. */
export function useWords(trackID?: number): Words | null {
  const [got, setGot] = useState<{ id: number; words: Words | null } | null>(null);
  useEffect(() => {
    if (!trackID) return;
    let on = true;
    api.lyrics(trackID).then((r) => {
      if (!on) return;
      const lines = parseLRC(r.lyrics ?? '');
      const plain = lines.length ? lines : plainLyrics(r.lyrics ?? '');
      setGot({ id: trackID, words: plain.length ? { lines: plain, timed: lines.length > 0 } : null });
    }).catch(() => {});
    return () => { on = false; };
  }, [trackID]);
  return trackID && got?.id === trackID ? got.words : null;
}

/** All of the words. The line being sung stays in view unless the reader has scrolled away. */
export function LyricSheet({ words, now, big }: { words: Words; now: Sounding | null; big: boolean }) {
  const box = useRef<HTMLOListElement>(null);
  const hands = useRef(0);
  const [i, setI] = useState(-1);
  const [free, setFree] = useState(false);
  const follow = !!now && words.timed;
  useEffect(() => {
    if (!follow) { setI(-1); return; }
    const f = () => {
      setI(lineAt(words.lines, position(now!)));
      setFree(Date.now() - hands.current < 5000);
    };
    f();
    const t = setInterval(f, 200);
    return () => clearInterval(t);
  }, [words, follow, now?.phase, now?.at, now?.since]); // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    const b = box.current, el = b?.children[Math.max(i, 0)] as HTMLElement | undefined;
    if (!b || !el || !follow || free) return;
    b.scrollTo({ top: i < 0 ? 0 : el.offsetTop + el.offsetHeight / 2 - b.clientHeight * .42, behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' });
  }, [i, big, follow, free]);
  const mine = () => { hands.current = Date.now(); if (follow) setFree(true); };
  return (
    <ol ref={box} className={`sheet-words ${follow ? 'follow' : ''} ${free ? 'free' : ''}`} aria-label="歌词" tabIndex={0} onWheel={mine} onTouchMove={mine} onKeyDown={mine}>
      {words.lines.map((l, k) => <li key={k} className={k === i ? 'now' : ''} aria-current={k === i || undefined} style={{ '--d': follow ? Math.min(5, Math.abs(k - Math.max(i, 0))) : 0 } as React.CSSProperties}>{l.text}</li>)}
    </ol>
  );
}

/** How far the song is. The fish swims at its head. */
export function Progress({ now, total }: { now: Sounding; total?: number }) {
  const [at, setAt] = useState(() => position(now));
  useEffect(() => {
    const f = () => setAt(position(now));
    f();
    const t = setInterval(f, 500);
    return () => clearInterval(t);
  }, [now.phase, now.at, now.since]); // eslint-disable-line react-hooks/exhaustive-deps
  const here = total ? Math.min(at, total) : at;
  return (
    <div className={`swim ${now.phase}`}>
      <span className="num">{length(here) || '0:00'}</span>
      <div style={{ '--f': total ? here / total : 0 } as React.CSSProperties}>{total ? <><i /><b><Fish mood={now.phase === 'playing' ? 'idle' : 'waiting'} /></b></> : null}</div>
      <span className="num">{total ? length(total) : ''}</span>
    </div>
  );
}
