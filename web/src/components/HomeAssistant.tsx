import { useEffect, useState } from 'react';
import { api, ApiError, type HomeStatus } from '../api';
import { HouseIcon, Setting, type Light } from './Setting';

/** The endpoint for Home Assistant: its switch and its token. */
export function HomeAssistant({ onLight }: { onLight: (l: Light) => void }) {
  const [st, setSt] = useState<HomeStatus | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [asking, setAsking] = useState(false);
  const [copied, setCopied] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    const load = () => api.home().then(setSt).catch(() => {});
    load();
    const t = setInterval(() => { if (!document.hidden) load(); }, 10000);
    return () => clearInterval(t);
  }, []);
  const light: Light = !st?.enabled ? 'off' : st.has_token ? 'ok' : 'warn';
  useEffect(() => { onLight(light); }, [light, onLight]);
  const work = async (f: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    try { await f(); } catch (e) { setError((e as ApiError).message); } finally { setBusy(false); }
  };
  const make = () => work(async () => {
    const r = await api.homeToken();
    setToken(r.token);
    setCopied(false);
    setAsking(false);
    setSt(await api.home());
  });
  const toggle = (on: boolean) => work(async () => {
    setSt(await api.setHome(on));
    if (!on) setToken(null);
  });
  const copy = async () => {
    try { await navigator.clipboard.writeText(token!); setCopied(true); } catch { setError('浏览器不让复制，请手动选中令牌复制'); }
  };
  return (
    <Setting id="home" icon={<HouseIcon />} title="Home Assistant" says="把音箱正在放的歌、歌词、封面和点歌口令实时给它，也能控制音箱" light={light}
      state={!st ? undefined : !st.enabled ? '已关闭' : !st.has_token ? '还没有令牌' : st.connected ? `已连接 ${st.connected} 个` : '等待连接'}
      actions={<label className="switch"><input type="checkbox" role="switch" checked={!!st?.enabled} disabled={!st || busy} onChange={(e) => toggle(e.target.checked)} aria-label="启用 Home Assistant 接口" /><span /></label>}>
      {st?.enabled && <>
        <dl className="kv">
          <div><dt>服务地址</dt><dd className="mono">{location.origin}</dd></div>
          <div><dt>令牌</dt><dd>{st.has_token ? '已设置，只用于这个接口' : '还没有生成'}</dd></div>
        </dl>
        {token && <div className="ha-token"><p>只显示这一次，复制后填到 Home Assistant：</p><code>{token}</code><button className="btn btn-small" onClick={copy}>{copied ? '已复制' : '复制'}</button></div>}
        {asking ? <div className="confirm"><p>换新后旧令牌立即失效，Home Assistant 要重新填写。</p><div className="card-actions"><button className="btn btn-danger" onClick={make} disabled={busy}>换新</button><button className="btn btn-quiet" onClick={() => setAsking(false)}>取消</button></div></div>
          : <div><button className={`btn ${st.has_token ? '' : 'btn-primary'}`} disabled={busy} onClick={() => (st.has_token ? setAsking(true) : make())}>{st.has_token ? '换新令牌' : '生成令牌'}</button></div>}
      </>}
      {error && <p className="field-error" role="alert">{error}</p>}
    </Setting>
  );
}
