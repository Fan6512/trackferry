import { useState } from 'react';
import { api, ApiError, type Device, type Mode, type WatchStatus } from '../api';
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
        <ModeSwitch device={device} onChange={onChange} />
        <p className="speaker-hint">
          {device.in_account ? modeHint[device.mode] : '设置会保留，重新绑定后恢复。'}
          {watch && device.mode !== 'off' && <small className={watch.state === 'error' ? 'bad' : ''}>{watchText[watch.state]}{watch.error ? `：${watch.error}` : watch.recorded > 0 ? `，启动后听到 ${watch.recorded} 次点歌` : ''}</small>}
        </p>
      </div>
    </li>
  );
}
