import { useEffect, useState } from 'react';
import { api, ApiError, type OnlineOutcome, type OnlineSong } from '../api';

export type OnlineFound = { songs: OnlineSong[]; sources: OnlineOutcome[] };
export const isDown = (s: OnlineOutcome) => s.state === 'error' || s.state === 'open';
const ONLINE_KEPT = 5 * 60_000;
const keptOnline = new Map<string, { at: number; res: OnlineFound }>();
/** What the online sources answered a short while ago. */
function answered(q: string) {
  const k = keptOnline.get(q);
  return k && Date.now() - k.at < ONLINE_KEPT ? k.res : null;
}

/** Songs the online sources know for q. They are asked only when the listener says so: asked is
    what for, and a higher turn asks again. */
export function useOnline(q: string, asked: string, turn: number) {
  const [res, setRes] = useState<OnlineFound | null>(() => answered(q));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let on = true;
    const had = q ? answered(q) : null;
    setRes(had);
    setError(null);
    setBusy(false);
    if (!q || had || asked !== q) return;
    setBusy(true);
    api.searchOnline(q).then((r) => {
      if (!r.sources.some(isDown)) {
        if (keptOnline.size >= 30) keptOnline.delete(keptOnline.keys().next().value!);
        keptOnline.set(q, { at: Date.now(), res: r });
      }
      if (on) { setRes(r); setBusy(false); }
    }, (e) => { if (on) { setError((e as ApiError).message); setBusy(false); } });
    return () => { on = false; };
  }, [q, asked, turn]);
  return { res, busy, error, down: res?.sources.filter(isDown) ?? [] };
}
