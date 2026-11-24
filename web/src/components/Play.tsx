import { useEffect, useRef, useState } from 'react';
import { api, ApiError, type Device } from '../api';
import { deviceTitle } from '../copy';

/** Plays one song of the library on a speaker that is connected. */
export function Play({ trackID, title, devices, label = false }: { trackID: number; title: string; devices: Device[]; label?: boolean }) {
  const targets = devices.filter((d) => d.in_account && d.mode !== 'off');
  const [open, setOpen] = useState(false);
  const [down, setDown] = useState(false);
  const [state, setState] = useState<{ tone: 'busy' | 'ok' | 'bad'; text: string } | null>(null);
  const box = useRef<HTMLSpanElement>(null);
  useEffect(() => {
    if (!open) return;
    const off = (e: Event) => { if (e instanceof KeyboardEvent ? e.key === 'Escape' : !box.current?.contains(e.target as Node)) setOpen(false); };
    addEventListener('pointerdown', off);
    addEventListener('keydown', off);
    return () => { removeEventListener('pointerdown', off); removeEventListener('keydown', off); };
  }, [open]);
  useEffect(() => {
    if (!state || state.tone === 'busy') return;
    const t = setTimeout(() => setState(null), 5000);
    return () => clearTimeout(t);
  }, [state]);

  const play = async (d: Device) => {
    setOpen(false);
    setState({ tone: 'busy', text: `正在送到${deviceTitle(d)}` });
    try {
      await api.playTrack(d.device_id, trackID);
      setState({ tone: 'ok', text: `已送往${deviceTitle(d)}` });
    } catch (e) {
      setState({ tone: 'bad', text: (e as ApiError).message });
    }
  };

  return (
    <span className={`play ${label ? 'play-label' : ''}`} ref={box}>
      <button type="button" className={label ? 'btn btn-primary' : 'play-btn'} disabled={state?.tone === 'busy'} aria-label={`播放${title}`} aria-expanded={open}
        onClick={(e) => { setDown(e.currentTarget.getBoundingClientRect().top < innerHeight / 2); setOpen(!open); }}>
        <svg viewBox="0 0 12 14" width="11" height="13" aria-hidden="true"><path d="M1 1l10 6-10 6z" fill="currentColor" /></svg>
        {label && '在音箱上播放'}
      </button>
      {open && (
        <span className={`menu ${down ? 'menu-down' : ''}`} role="menu">
          <span className="menu-head">在哪里播放</span>
          {targets.length === 0 && <span className="menu-empty">没有接入的音箱</span>}
          {targets.map((d) => <button key={d.device_id} role="menuitem" type="button" onClick={() => play(d)}><svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true"><rect x="6" y="2.5" width="12" height="19" rx="5" fill="none" stroke="currentColor" strokeWidth="1.5" /><path d="M9 6.5h6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" /><circle cx="12" cy="14.5" r="2.6" fill="none" stroke="currentColor" strokeWidth="1.5" /></svg><span>{deviceTitle(d)}</span></button>)}
        </span>
      )}
      {state && <span className={`play-state ${state.tone}`} role="status">{state.text}</span>}
    </span>
  );
}
