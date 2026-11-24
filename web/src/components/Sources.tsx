import { useCallback, useEffect, useState } from 'react';
import { api, ApiError, type SourceHealth } from '../api';
import { clock } from '../copy';
import { Setting, WavesIcon, type Light } from './Setting';
import { SourceIcon } from './SourceIcon';
import { ms, stopLabel } from '../route';

export function Sources({ onLight }: { onLight: (l: Light) => void }) {
  const [sources, setSources] = useState<SourceHealth[]>([]);
  const [error, setError] = useState<string | null>(null);
  const load = useCallback(() => api.library().then((r) => setSources(r.sources ?? [])).catch((e) => setError((e as ApiError).message)), []);
  useEffect(() => { load(); }, [load]);
  const set = async (s: SourceHealth, on: boolean) => {
    try {
      setSources((await api.setSource(s.source, on)).sources);
      setError(null);
    } catch (e) {
      setError((e as ApiError).message);
    }
  };
  const on = sources.filter((s) => s.enabled).length;
  const down = (s: SourceHealth) => s.enabled && (s.open_until_ms ?? 0) > Date.now();
  const light: Light | null = !sources.length ? null : sources.some(down) ? 'warn' : on ? 'ok' : 'off';
  useEffect(() => { if (light) onLight(light); }, [light, onLight]);
  return (
    <Setting id="sources" icon={<WavesIcon />} title="线上音源" says="曲库没有时依次去找，只取免费的完整版" light={light ?? 'off'} state={sources.length ? on ? `启用 ${on} 个` : '都关着' : undefined}>
      <ul className="sources">
        {sources.map((s, i) => {
          const all = s.ok + s.failed;
          return (
            <li key={s.source} className={s.enabled ? '' : 'off'}>
              <span className="source-no num">{i + 1}</span>
              <SourceIcon name={s.source} />
              <span className="source-state"><b>{stopLabel[s.source]}</b><small>{!s.enabled ? '已关闭' : down(s) ? <span className="bad">连续失败，{clock(s.open_until_ms!).slice(0, 5)} 后再试</span> : s.last_at_ms ? `上次 ${clock(s.last_at_ms).slice(0, 5)}` : '启动后还没用过'}{s.last_error && s.enabled ? `，${s.last_error}` : ''}</small></span>
              <span className="source-time num">{s.enabled && s.last_at_ms ? ms(s.last_ms) : ''}</span>
              <span className="source-rate">{all > 0 && <><i><i style={{ width: `${s.ok / all * 100}%` }} /></i><small className="num">{s.ok} / {all}</small></>}</span>
              <label className="switch"><input type="checkbox" role="switch" checked={s.enabled} onChange={(e) => set(s, e.target.checked)} aria-label={`使用${stopLabel[s.source]}`} /><span /></label>
            </li>
          );
        })}
      </ul>
      {error && <p className="field-error">{error}</p>}
    </Setting>
  );
}
