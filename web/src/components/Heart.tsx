import { useEffect, useState } from 'react';
import { api, ApiError } from '../api';

/** Puts a song into the favourites of 飞牛音乐. */
export function Heart({ trackID, title, on }: { trackID: number; title: string; on?: boolean }) {
  const [now, setNow] = useState(!!on);
  const [busy, setBusy] = useState(false);
  const [bad, setBad] = useState('');
  useEffect(() => {
    if (on !== undefined) { setNow(on); return; }
    let live = true;
    api.isFavorite(trackID).then((r) => live && setNow(r.favorite), () => {});
    return () => { live = false; };
  }, [trackID, on]);
  const flip = async () => {
    setBusy(true);
    setBad('');
    try {
      setNow((await api.favorite(trackID, !now)).favorite);
    } catch (e) {
      setBad((e as ApiError).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <button type="button" className={`heart ${now ? 'on' : ''}`} disabled={busy} aria-pressed={now} aria-label={`${now ? '取消收藏' : '收藏'}${title}`} title={bad || undefined} onClick={flip}>
      <svg viewBox="0 0 20 18" width="18" height="16" aria-hidden="true"><path d="M10 16.6S1.4 11.6 1.4 5.9C1.4 3.3 3.4 1.4 5.8 1.4c1.8 0 3.3 1 4.2 2.6.9-1.6 2.4-2.6 4.2-2.6 2.4 0 4.4 1.9 4.4 4.5 0 5.7-8.6 10.7-8.6 10.7z" fill={now ? 'currentColor' : 'none'} stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" /></svg>
    </button>
  );
}
