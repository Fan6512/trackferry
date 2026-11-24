import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { api, ApiError, type Controls, type Device, type Hush, type Mode, type PlayMode, type Press, type WatchStatus } from '../api';
import { onSpeaker, useLive } from '../live';
import { Pick } from './Pick';
import { deviceSubtitle, deviceTitle, isOnline, modeHint, modeLabel } from '../copy';

const modes: Mode[] = ['off', 'observe', 'takeover'];

export const watchText: Record<WatchStatus['state'], string> = {
  starting: '正在连接', listening: '正在听', error: '读取失败', paused: '已暂停', unsupported: '这个型号暂不支持',
};

function ModeSwitch({ device, onChange }: { device: Device; onChange: (d: Device) => void }) {
  const [busy, setBusy] = useState<Mode | null>(null);
  const [error, setError] = useState<string | null>(null);
  const set = async (mode: Mode) => {
    if (mode === device.mode || busy) return;
    setBusy(mode);
    setError(null);
    try {
      onChange(await api.setMode(device.device_id, mode));
    } catch (e) {
      setError((e as ApiError).message);
    } finally {
      setBusy(null);
    }
  };
  return (
    <div className="mode">
      <div className="segmented" role="radiogroup" aria-label={`${deviceTitle(device)} 的模式`}>
        {modes.map((m) => {
          const locked = m === 'takeover' && !device.takeover_allowed && device.mode !== 'takeover';
          return (
            <button
              key={m}
              role="radio"
              aria-checked={device.mode === m}
              className={`seg seg-${m} ${device.mode === m ? 'on' : ''}`}
              disabled={locked || !device.in_account}
              title={locked ? '未在 config.yaml 的 takeover_speakers 中放行' : modeHint[m]}
              onClick={() => set(m)}
            >
              {busy === m && <span className="spinner spinner-tiny" />}
              {modeLabel[m]}
              {locked && <svg className="lock" viewBox="0 0 10 12" width="9" height="11" aria-label="未放行"><path d="M2.5 5V3.5a2.5 2.5 0 015 0V5M1 5h8v6H1z" fill="none" stroke="currentColor" strokeWidth="1.5" /></svg>}
            </button>
          );
        })}
      </div>
      {error && <p className="field-error">{error}</p>}
    </div>
  );
}

const hushes: { value: Hush; label: string; says: string }[] = [
  { value: 'normal', label: '普通', says: '音量调到零并打断，出歌最快' },
  { value: 'strong', label: '加强', says: '再加一道米家静音和暂停，适合压不住的型号' },
  { value: 'wait', label: '等小爱说完', says: '一直静音，估计小爱说完了再放歌，出歌慢一些' },
];

function HushPick({ device, onChange }: { device: Device; onChange: (d: Device) => void }) {
  const [error, setError] = useState<string | null>(null);
  const set = async (h: Hush) => {
    try { onChange(await api.setHush(device.device_id, h)); setError(null); } catch (e) { setError((e as ApiError).message); }
  };
  return (
    <div className="hush">
      <span>压住小爱的回答</span>
      <Pick label={`${deviceTitle(device)}压住小爱回答的方式`} value={device.hush} choices={hushes.filter((h) => h.value !== 'strong' || device.can_hush)} onPick={set} />
      <small>{hushes.find((h) => h.value === device.hush)?.says}</small>
      {error && <p className="field-error" role="alert">{error}</p>}
    </div>
  );
}

/** The speaker itself; its ring shows what it is doing. */
function Body({ mode, state }: { mode: Mode; state?: WatchStatus['state'] }) {
  const on = mode !== 'off' && state === 'listening';
  return (
    <svg className={`body body-${mode} ${on ? 'body-on' : ''}`} viewBox="0 0 120 170" aria-hidden="true">
      <defs>
        <linearGradient id="shell" x1="0" x2="1"><stop stopColor="#0b0e11" /><stop offset=".28" stopColor="#4a535b" /><stop offset=".55" stopColor="#262c32" /><stop offset="1" stopColor="#07090b" /></linearGradient>
        <pattern id="mesh" width="5" height="5" patternUnits="userSpaceOnUse" patternTransform="rotate(45)"><circle cx="2.5" cy="2.5" r="1.1" fill="#000" fillOpacity=".5" /></pattern>
      </defs>
      <ellipse cx="60" cy="156" rx="46" ry="9" fill="#000" fillOpacity=".45" />
      <path d="M14 26v118c0 9 20 16 46 16s46-7 46-16V26z" fill="url(#shell)" />
      <path d="M14 40v104c0 9 20 16 46 16s46-7 46-16V40c-9 8-27 12-46 12s-37-4-46-12z" fill="url(#mesh)" />
      <ellipse cx="60" cy="26" rx="46" ry="15" fill="#15191d" />
      <ellipse className="ring" cx="60" cy="26" rx="36" ry="11" fill="none" strokeWidth="3.5" />
    </svg>
  );
}

export function SpeakerRow({ device, watch, onChange }: { device: Device; watch?: WatchStatus; onChange: (d: Device) => void }) {
  return (
    <li className={`speaker mode-${device.mode} ${device.in_account ? '' : 'speaker-gone'}`}>
      <Body mode={device.in_account ? device.mode : 'off'} state={watch?.state} />
      <div className="speaker-info">
        <h2>{deviceTitle(device)}</h2>
        <p className="speaker-meta">
          {device.in_account
            ? <span className={isOnline(device) ? 'on-dot' : 'off-dot'}>{isOnline(device) ? '在线' : '离线'}</span>
            : <span className="off-dot">已不在账号中</span>}
          <span>{deviceSubtitle(device)}</span>
        </p>
        {device.in_account && device.mode !== 'off' && <Buttons device={device} />}
        <ModeSwitch device={device} onChange={onChange} />
        {device.in_account && device.mode === 'takeover' && <HushPick device={device} onChange={onChange} />}
        <p className="speaker-hint">
          {device.in_account ? modeHint[device.mode] : '设置会保留，重新绑定后恢复。'}
          {watch && device.mode !== 'off' && <small className={watch.state === 'error' ? 'bad' : ''}>{watchText[watch.state]}{watch.error ? `：${watch.error}` : watch.recorded > 0 ? `，启动后听到 ${watch.recorded} 次点歌` : ''}</small>}
        </p>
      </div>
    </li>
  );
}

const draw = (d: string, fill = true) => <svg viewBox="0 0 24 24" width="20" height="20" fill={fill ? 'currentColor' : 'none'} stroke={fill ? 'none' : 'currentColor'} strokeWidth="1.7" strokeLinecap="round" aria-hidden="true"><path d={d} /></svg>;
const icons = {
  play: draw('M8 5.2v13.6L19 12z'), pause: draw('M7 5h3.6v14H7zM13.4 5H17v14h-3.6z'), stop: draw('M6.5 6.5h11v11h-11z'),
  previous: draw('M6.2 5.5h2.2v13H6.2zM18.6 5.8v12.4L9.4 12z'), next: draw('M15.6 5.5h2.2v13h-2.2zM5.4 5.8v12.4L14.6 12z'),
  wake: draw('M12 4a2.6 2.6 0 0 1 2.6 2.6v4.8a2.6 2.6 0 0 1-5.2 0V6.6A2.6 2.6 0 0 1 12 4zM6 11.5a6 6 0 0 0 12 0M12 17.5V21', false),
  volume: draw('M4 9.5v5h3.5L12 18.5v-13L7.5 9.5zM15.5 9a4.2 4.2 0 0 1 0 6M18 6.5a7.8 7.8 0 0 1 0 11', false),
};

const playModes: Record<PlayMode, { name: string; icon: ReactNode; next: PlayMode }> = {
  shuffle: { name: '随机播放', next: 'loop', icon: draw('M4 7h2.6c1.5 0 2.6.8 3.5 2.2l3.8 5.6c.9 1.4 2 2.2 3.5 2.2H20M4 17h2.6c1.5 0 2.6-.8 3.5-2.2M13.9 9.2C14.8 7.8 15.9 7 17.4 7H20M17.5 4.5 20 7l-2.5 2.5M17.5 14.5 20 17l-2.5 2.5', false) },
  loop: { name: '顺序循环', next: 'one', icon: draw('M4.5 11.5V10A3.5 3.5 0 0 1 8 6.5h11.5M17 4l2.5 2.5L17 9M19.5 12.5V14a3.5 3.5 0 0 1-3.5 3.5H4.5M7 15l-2.5 2.5L7 20', false) },
  one: { name: '单曲循环', next: 'shuffle', icon: draw('M4.5 11.5V10A3.5 3.5 0 0 1 8 6.5h11.5M17 4l2.5 2.5L17 9M19.5 12.5V14a3.5 3.5 0 0 1-3.5 3.5H4.5M7 15l-2.5 2.5L7 20M11 10.6l1.4-.9v4.8', false) },
};

/** What a speaker is to do: mode, previous, play, next, stop; wake and volume. */
export function Buttons({ device, name }: { device: Device; name?: string }) {
  const id = device.device_id;
  const flow = onSpeaker(useLive().flows, id);
  const phase = flow?.player?.phase;
  const [meant, setMeant] = useState<'playing' | 'paused' | 'idle' | null>(null); // until the speaker is heard from
  const [now, setNow] = useState<Controls | null>(null);
  const [drag, setDrag] = useState<number | null>(null);
  const [busy, setBusy] = useState<Press | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [said, setSaid] = useState<string | null>(null);
  const again = useRef<ReturnType<typeof setTimeout>>(undefined);
  const alive = useRef(true);
  const load = useCallback(async () => {
    try { const s = await api.controls(id); if (alive.current) setNow(s); } catch { /* the buttons work without it */ }
  }, [id]);
  useEffect(() => {
    alive.current = true;
    load();
    const t = setInterval(() => { if (!document.hidden) load(); }, 8000);
    return () => { alive.current = false; clearInterval(t); clearTimeout(again.current); };
  }, [load]);
  useEffect(() => { setMeant(null); load(); }, [phase, flow?.id, load]);
  useEffect(() => {
    if (!meant) return;
    const t = setTimeout(() => setMeant(null), 5000);
    return () => clearTimeout(t);
  }, [meant]);
  const state = meant ?? (phase ? (phase === 'paused' ? 'paused' : 'playing') : now?.state);
  useEffect(() => {
    if (!said) return;
    const t = setTimeout(() => setSaid(null), 4000);
    return () => clearTimeout(t);
  }, [said]);
  const press = async (action: Press, set: { volume?: number; mode?: PlayMode } = {}) => {
    setBusy(action);
    setError(null);
    setSaid(null);
    const to = ({ play: 'playing', pause: 'paused', stop: 'idle' } as const)[action as 'play'];
    if (to) { setMeant(to); setNow((n) => n && { ...n, state: to }); }
    try {
      await api.press(id, action, set);
      if (action === 'wake') setSaid('已唤醒，直接说话就行');
      if (set.mode) setSaid(playModes[set.mode].name);
      setNow((n) => n && { ...n, ...set });
    } catch (e) {
      setMeant(null);
      setError((e as ApiError).message);
    } finally {
      setBusy((b) => (b === action ? null : b));
      setDrag(null);
      clearTimeout(again.current);
      again.current = setTimeout(load, 900); // what was pressed takes a moment
    }
  };
  const commit = () => { if (drag !== null && drag !== now?.volume) press('volume', { volume: drag }); else setDrag(null); };
  const button = (a: Press, label: string, off = false, main = false) => <button type="button" className={main ? 'ctl ctl-main' : 'ctl'} onClick={() => press(a)} disabled={off || busy === a} title={label} aria-label={`${deviceTitle(device)}${label}`}>{busy === a ? <span className="spinner spinner-tiny" /> : icons[a as keyof typeof icons]}</button>;
  const mode = now?.mode ? playModes[now.mode] : null;
  const alone = now !== null && !now.queue; // nothing of ours to move in
  const list = now?.queue?.list;
  const volume = drag ?? now?.volume ?? null;
  return (
    <div className="ctls">
      <div className="ctl-row">
        {(name || list) && <span className="ctl-name">{name && <b>{name}</b>}{list && <span>{list}</span>}{list && now?.queue && <small className="num">{now.queue.at} / {now.queue.total}</small>}</span>}
        <div className="ctl-play" role="group" aria-label="播放控制">
          {mode && <button type="button" className="ctl ctl-bare" onClick={() => press('mode', { mode: mode.next })} disabled={busy === 'mode'} title={mode.name} aria-label={`${deviceTitle(device)}播放方式：${mode.name}，点一下换成${playModes[mode.next].name}`}>{mode.icon}</button>}
          {button('previous', '上一首', alone)}
          {state === 'playing' ? button('pause', '暂停', false, true) : button('play', '播放', false, true)}
          {button('next', '下一首', alone)}
          {button('stop', '停止')}
        </div>
        {device.can_wake && button('wake', '唤醒')}
        <label className="ctl-vol">
          {icons.volume}
          <input type="range" min={0} max={100} value={volume ?? 0} disabled={volume === null} aria-label={`${deviceTitle(device)}音量`}
            onChange={(e) => setDrag(Number(e.target.value))} onPointerUp={commit} onKeyUp={commit} onBlur={commit} />
          <small className="num">{volume ?? '–'}</small>
        </label>
      </div>
      {error && <p className="field-error" role="alert">{error}</p>}
      {said && <p className="ctl-said" role="status">{said}</p>}
    </div>
  );
}
