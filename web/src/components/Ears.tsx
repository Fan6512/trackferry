import { useEffect, useRef } from 'react';
import type { Device } from '../api';
import { deviceTitle } from '../copy';
import { connected } from '../ear';
import { onSpeaker, phaseText, useLive } from '../live';
import { Cover } from './Cover';
import { watchText } from './Speaker';

function Body() {
  return <svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true"><rect x="6" y="2.5" width="12" height="19" rx="5" fill="none" stroke="currentColor" strokeWidth="1.5" /><path d="M9.4 6.5h5.2" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" /><circle cx="12" cy="14.5" r="2.6" fill="none" stroke="currentColor" strokeWidth="1.5" /></svg>;
}

/** Chooses which speaker to look at. */
export function Ears({ devices, value, onChange }: { devices: Device[]; value: string; onChange: (id: string) => void }) {
  const live = useLive();
  const list = connected(devices);
  const box = useRef<HTMLDivElement>(null);
  // The chosen one may lie off the edge of a narrow rail.
  useEffect(() => {
    const b = box.current, a = b?.querySelector<HTMLElement>('[aria-pressed=true]');
    if (b && a) b.scrollTo({ left: a.offsetLeft - (b.clientWidth - a.offsetWidth) / 2, behavior: 'smooth' });
  }, [value, list.length]);
  if (list.length < 2) return null;
  const busy = list.filter((d) => onSpeaker(live.flows, d.device_id)).length;
  return (
    <div className="spks" ref={box} role="group" aria-label="看哪台音箱">
      <button type="button" aria-pressed={!value} onClick={() => onChange('')}>
        <i><svg viewBox="0 0 24 24" width="22" height="22" aria-hidden="true"><path d="M5 15V9M10 19V5M15 16V8M20 13.5v-3" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" /></svg></i>
        <span><b>全部</b><small>{busy ? `${busy} 台在放` : `${list.length} 台`}</small></span>
      </button>
      {list.map((d) => {
        const f = onSpeaker(live.flows, d.device_id);
        const song = f?.route?.decision !== 'native' && f?.route?.title ? f.route : f?.run?.song ?? null;
        const p = live.pulses[d.device_id];
        const ph = f?.player?.phase;
        return (
          <button key={d.device_id} type="button" aria-pressed={value === d.device_id} className={ph === 'playing' ? 'playing' : ''} onClick={() => onChange(d.device_id)}>
            <i>{song ? <Cover cover={song.cover} title={song.title ?? ''} artist={song.artist} /> : <Body />}</i>
            <span><b>{deviceTitle(d)}</b><small>{song ? `${ph !== 'playing' ? `${phaseText[ph!]}　` : ''}${song.title}` : p ? watchText[p.state] : '没有在听'}</small></span>
            {ph === 'playing' && <em className="eq" aria-hidden="true"><u /><u /><u /></em>}
          </button>
        );
      })}
    </div>
  );
}
