import { useState } from 'react';
import type { Device } from '../api';
import { deviceTitle } from '../copy';
import { connected } from '../ear';
import { onSpeaker, phaseText, useLive } from '../live';
import { Cover } from './Cover';
import { Sheet } from './Sheet';
import { watchText } from './Speaker';

export const speakerIcon = <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><rect x="6" y="2.5" width="12" height="19" rx="5" fill="none" stroke="currentColor" strokeWidth="1.6" /><path d="M9.4 6.5h5.2" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" /><circle cx="12" cy="14.5" r="2.6" fill="none" stroke="currentColor" strokeWidth="1.6" /></svg>;
const allIcon = <svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true"><path d="M5 15V9M10 19V5M15 16V8M20 13.5v-3" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" /></svg>;

/** Which speaker it is about, in a word; a tap changes it. all: "" may be chosen too. */
export function SpeakerChip({ devices, value, onChange, all = false, title = '看哪台音箱', bare = '' }: { devices: Device[]; value: string; onChange: (id: string) => void; all?: boolean; title?: string; /** A word without a button around it; what it says when none is chosen. */ bare?: string }) {
  const { flows, pulses } = useLive();
  const [open, setOpen] = useState(false);
  const list = connected(devices);
  if (!list.length) return null;
  const d = list.find((x) => x.device_id === value);
  const on = (id: string) => onSpeaker(flows, id)?.player?.phase === 'playing';
  const busy = d ? on(d.device_id) : list.some((x) => on(x.device_id));
  const alone = list.length < 2;
  const pick = (id: string) => { onChange(id); setOpen(false); };
  return (
    <>
      <button type="button" className={bare ? 'chip chip-bare' : 'chip glass'} disabled={alone} onClick={() => setOpen(true)} aria-haspopup="dialog" aria-label={`${title}：${d ? deviceTitle(d) : '全部'}`}>
        {busy ? <em className="eq" aria-hidden="true"><u /><u /><u /></em> : d || alone ? speakerIcon : allIcon}
        <b>{d ? deviceTitle(d) : alone ? deviceTitle(list[0]) : bare || '全部音箱'}</b>
        {!alone && <svg viewBox="0 0 12 8" width="11" height="8" aria-hidden="true"><path d="M1 1.5l5 5 5-5" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" /></svg>}
      </button>
      {open && (
        <Sheet title={title} onClose={() => setOpen(false)}>
          <div className="picks">
            {all && <button type="button" aria-pressed={!value} onClick={() => pick('')}><i>{allIcon}</i><span><b>全部音箱</b><small>{list.length} 台一起看</small></span></button>}
            {list.map((x) => {
              const f = onSpeaker(flows, x.device_id);
              const song = f?.route?.decision !== 'native' && f?.route?.title ? f.route : f?.run?.song ?? null;
              const p = pulses[x.device_id];
              return (
                <button key={x.device_id} type="button" aria-pressed={value === x.device_id} onClick={() => pick(x.device_id)}>
                  <i>{song ? <Cover cover={song.cover} title={song.title ?? ''} artist={song.artist} /> : speakerIcon}</i>
                  <span><b>{deviceTitle(x)}</b><small>{song ? `${f?.player?.phase && f.player.phase !== 'playing' ? `${phaseText[f.player.phase]}　` : ''}${song.title}` : p ? watchText[p.state] : '没有在听'}</small></span>
                  {on(x.device_id) && <em className="eq" aria-hidden="true"><u /><u /><u /></em>}
                </button>
              );
            })}
          </div>
        </Sheet>
      )}
    </>
  );
}
