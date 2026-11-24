import { useState, type FormEvent, type ReactNode } from 'react';
import { api, ApiError } from '../api';
import { useEffect, useRef } from 'react';
import { useWater, fishPicture } from '../stage/Stage';
import { Fish } from '../components/Fish';

function AuthShell({ title, lead, children }: { title: string; lead: ReactNode; children: ReactNode }) {
  const water = useWater();
  const raft = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!water) return;
    water.show(fishPicture);
    const set = () => { const bounds = raft.current!.getBoundingClientRect(); water.place(null, bounds.bottom - 2); };
    set();
    addEventListener('resize', set);
    return () => removeEventListener('resize', set);
  }, [water]);
  return (
    <main className="auth" onPointerDown={(e) => water?.ripple(e.clientX, e.clientY)}>
      <div className="auth-raft" ref={raft}><Fish water /></div>
      <div className="auth-card">
        <h1>{title}</h1>
        <p className="lead">{lead}</p>
        {children}
      </div>
    </main>
  );
}

export function Setup({ onDone }: { onDone: () => void }) {
  const [code, setCode] = useState('');
  const [pw, setPw] = useState('');
  const [pw2, setPw2] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (pw.length < 8) return setError('密码至少 8 位');
    if (pw !== pw2) return setError('两次输入的密码不一样');
    setBusy(true);
    setError(null);
    try {
      await api.setup(code, pw);
      onDone();
    } catch (err) {
      setError((err as ApiError).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <AuthShell title="设置管理密码" lead="只用来打开这个页面，和小米账号无关。">
      <form className="form" onSubmit={submit}>
        <label className="field">
          <span>初始化码</span>
          <input value={code} onChange={(e) => setCode(e.target.value)} autoComplete="off" autoCapitalize="characters" spellCheck={false} placeholder="8 位字母和数字" required />
          <small>
            在容器日志的最后，也可以在 NAS 上运行
            <code>docker exec trackferry /trackferry setup-code</code>
          </small>
        </label>
        <label className="field">
          <span>管理密码</span>
          <input type="password" value={pw} onChange={(e) => setPw(e.target.value)} autoComplete="new-password" minLength={8} required />
        </label>
        <label className="field">
          <span>再输入一次</span>
          <input type="password" value={pw2} onChange={(e) => setPw2(e.target.value)} autoComplete="new-password" minLength={8} required />
        </label>
        {error && <p className="field-error" role="alert">{error}</p>}
        <button className="btn btn-primary btn-block" disabled={busy}>{busy ? '正在保存…' : '设置并进入'}</button>
      </form>
    </AuthShell>
  );
}

const KEEP = 'trackferry.keep';
const remembered = () => { try { return localStorage.getItem(KEEP) !== 'off'; } catch { return true; } };

/** Gives the password to the browser's own store; the page itself never writes it down. */
async function handOver(password: string) {
  const Credential = (window as { PasswordCredential?: new (c: { id: string; name: string; password: string }) => globalThis.Credential }).PasswordCredential;
  if (!Credential || !navigator.credentials) return;
  await Promise.race([navigator.credentials.store(new Credential({ id: 'TrackFerry', name: 'TrackFerry', password })).catch(() => {}), new Promise((r) => setTimeout(r, 800))]);
}

export function Login({ onDone }: { onDone: () => void }) {
  const [pw, setPw] = useState('');
  const [keep, setKeep] = useState(remembered);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.login(pw, keep);
      try { localStorage.setItem(KEEP, keep ? 'on' : 'off'); } catch { /* asked again next time */ }
      if (keep) await handOver(pw);
      onDone();
    } catch (err) {
      setError((err as ApiError).message);
      setPw('');
    } finally {
      setBusy(false);
    }
  };
  return (
    <AuthShell title="TrackFerry" lead="">
      <form className="form" onSubmit={submit} autoComplete={keep ? 'on' : 'off'}>
        <input className="login-who" type="text" name="username" value="TrackFerry" autoComplete="username" readOnly tabIndex={-1} aria-hidden="true" />
        <label className="field">
          <span>管理密码</span>
          <input type="password" name="password" value={pw} onChange={(e) => setPw(e.target.value)} autoComplete={keep ? 'current-password' : 'off'} autoFocus required />
        </label>
        <label className="login-keep"><input type="checkbox" checked={keep} onChange={(e) => setKeep(e.target.checked)} /><span>保存密码<small>{keep ? '由浏览器保存，180 天内不用再登录' : '关掉浏览器就要重新登录'}</small></span></label>
        {error && <p className="field-error" role="alert">{error}</p>}
        <button className="btn btn-primary btn-block" disabled={busy || !pw}>{busy ? '正在登录…' : '登录'}</button>
        <p className="form-foot">忘记密码：在 NAS 上运行 <code>docker exec trackferry /trackferry reset-admin</code>，再用它给出的新初始化码重设。</p>
      </form>
    </AuthShell>
  );
}
