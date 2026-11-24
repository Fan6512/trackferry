import { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { api, ApiError, type NeteaseAccount, type NeteaseQR } from '../api';
import { qr } from '../qr';
import './netease.css';

/** The code as one path, with its quiet zone. */
function Code({ text }: { text: string }) {
  const m = qr(text);
  const d = m.flatMap((row, y) => row.map((on, x) => (on ? `M${x + 4} ${y + 4}h1v1h-1z` : ''))).join('');
  return <svg viewBox={`0 0 ${m.length + 8} ${m.length + 8}`} shapeRendering="crispEdges" role="img" aria-label="网易云登录二维码"><rect width="100%" height="100%" fill="#fff" /><path d={d} fill="#0f181b" /></svg>;
}

/** The session of a browser that is signed in already. */
function Paste({ onDone }: { onDone: (a: NeteaseAccount) => void }) {
  const [text, setText] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const send = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true); setError(null);
    try {
      onDone(await api.neteaseSignIn(text));
    } catch (x) {
      setError((x as ApiError).message);
      setBusy(false);
    }
  };
  return (
    <form className="ne-paste" onSubmit={send}>
      <ol>
        <li>电脑浏览器打开 <a href="https://music.163.com" target="_blank" rel="noreferrer">music.163.com</a> 并登录</li>
        <li>按 F12，进“应用程序 → Cookie”</li>
        <li>复制 MUSIC_U 那一行的值，贴到这里</li>
      </ol>
      <label className="field"><span>MUSIC_U</span><input type="password" value={text} onChange={(e) => setText(e.target.value)} autoComplete="off" spellCheck={false} required /></label>
      {error && <p className="field-error" role="alert">{error}</p>}
      <button className="btn btn-primary" disabled={busy || !text.trim()}>{busy ? '正在核对…' : '登录'}</button>
    </form>
  );
}

function Login({ onDone, onClose }: { onDone: (a: NeteaseAccount) => void; onClose: () => void }) {
  const box = useRef<HTMLDialogElement>(null);
  const [url, setUrl] = useState('');
  const [state, setState] = useState<NeteaseQR['state']>('waiting');
  const [error, setError] = useState<string | null>(null);
  const [paste, setPaste] = useState(false);
  const [round, setRound] = useState(0);
  useEffect(() => {
    box.current?.showModal();
    return () => { api.neteaseCancel().catch(() => {}); };
  }, []);
  useEffect(() => {
    if (paste) return;
    let off = false, t = 0;
    setUrl(''); setState('waiting'); setError(null);
    const ask = async () => {
      try {
        const r = await api.neteaseQR();
        if (off) return;
        setState(r.state);
        if (r.state === 'done') return onDone(r.account);
        if (r.state === 'expired') return;
      } catch (e) {
        if (off) return;
        setError((e as ApiError).message);
        return;
      }
      t = window.setTimeout(ask, 2000);
    };
    api.neteaseStart().then((r) => { if (!off) { setUrl(r.url); t = window.setTimeout(ask, 2000); } }, (e) => { if (!off) setError((e as ApiError).message); });
    return () => { off = true; clearTimeout(t); };
    // eslint-disable-next-line
  }, [round, paste]);
  const stale = state === 'expired' || !!error;
  return createPortal(
    <dialog className="ne-login" ref={box} onClose={onClose} onClick={(e) => { if (e.target === box.current) box.current?.close(); }}>
      <div>
        <h3>登录网易云音乐</h3>
        {paste ? <Paste onDone={onDone} /> : <>
        <div className={`ne-code ${stale ? 'stale' : ''}`}>
          {url && <Code text={url} />}
          {stale && <button type="button" className="btn btn-small" onClick={() => setRound(round + 1)}>换一张</button>}
        </div>
        <p role="status">{error ?? (state === 'expired' ? '二维码过期了' : state === 'scanned' ? '扫到了，在手机上点确认' : url ? '用网易云音乐 App 扫一扫' : '正在取二维码')}</p>
        </>}
        <button type="button" className="btn btn-small btn-quiet" onClick={() => setPaste(!paste)}>{paste ? '改用扫码' : '扫码登不上？粘贴凭据'}</button>
        <small>登录凭据只留在这台机器的 data/private</small>
        <button type="button" className="btn btn-small btn-quiet" onClick={() => box.current?.close()}>关闭</button>
      </div>
    </dialog>, document.body);
}

/** One line under 网易云: whose account it plays with. */
export function Netease() {
  const [a, setA] = useState<NeteaseAccount | null>(null);
  const [open, setOpen] = useState(false);
  const [sure, setSure] = useState(false);
  useEffect(() => { api.netease().then(setA).catch(() => {}); }, []);
  const leave = () => api.neteaseLogout().then((x) => { setA(x); setSure(false); }, () => setSure(false));
  if (!a) return null;
  return (
    <small className="ne">
      {a.linked && !a.expired ? <span className="ne-name">{a.name || '已登录'}{a.vip ? '，会员' : ''}</span> : <span className={a.expired ? 'bad' : ''}>{a.expired ? '登录失效，只放免费的' : '没登录，只放免费的'}</span>}
      {a.linked && !a.expired
        ? sure ? <><button type="button" onClick={leave}>确认退出</button><button type="button" onClick={() => setSure(false)}>取消</button></> : <button type="button" onClick={() => setSure(true)}>退出</button>
        : <button type="button" onClick={() => setOpen(true)}>{a.expired ? '重新登录' : '扫码登录'}</button>}
      {open && <Login onDone={(x) => { setA(x); setOpen(false); }} onClose={() => setOpen(false)} />}
    </small>
  );
}
