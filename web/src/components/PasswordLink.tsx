import { useEffect, useRef, useState, type FormEvent } from 'react';
import { api, ApiError, captchaURL, type LinkResult, type PasswordStatus } from '../api';

/**
 * Xiaomi account + password login, with the image captcha and SMS / email check Xiaomi's
 * risk control may add. The password is sent once to TrackFerry, used for this login only,
 * and never stored.
 */
export function PasswordLink({ onLinked }: { onLinked: (r: LinkResult) => void }) {
  const [st, setSt] = useState<PasswordStatus | null>(null);
  const [account, setAccount] = useState('');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const resumed = useRef(false);

  useEffect(() => {
    if (resumed.current) return;
    resumed.current = true;
    api.passwordStatus().then((s) => s.state !== 'linked' && setSt(s)).catch(() => {});
  }, []);

  const run = async (f: () => Promise<PasswordStatus>, keepCode = false) => {
    setBusy(true);
    setError(null);
    try {
      const next = await f();
      setPassword('');
      if (!keepCode) setCode('');
      if (next.state === 'linked') {
        setSt(null);
        onLinked(next);
        return;
      }
      setSt(next);
    } catch (e) {
      const err = e as ApiError;
      setError(err.message);
      // The attempt is gone on the server: go back to the account form.
      if (err.status === 409) setSt(null);
    } finally {
      setBusy(false);
    }
  };

  const start = (e: FormEvent) => {
    e.preventDefault();
    run(() => api.startPassword(account.trim(), password));
  };
  const restart = () => {
    api.cancelPassword().catch(() => {});
    setSt(null);
    setCode('');
    setError(null);
  };

  if (st?.state === 'need_captcha') {
    return (
      <form className="form pw" onSubmit={(e) => { e.preventDefault(); run(() => api.submitCaptcha(code)); }}>
        <p className="pw-step">正在登录 <b>{st.account}</b> · 小米要求输入图形验证码</p>
        <div className="captcha">
          <img src={captchaURL(st.captcha_rev ?? 0)} alt="图形验证码" />
          <button type="button" className="link" onClick={() => run(() => api.refreshCaptcha())} disabled={busy}>看不清，换一张</button>
        </div>
        <label className="field">
          <span>图中的字符</span>
          <input value={code} onChange={(e) => setCode(e.target.value)} autoComplete="off" autoCapitalize="off" spellCheck={false} required autoFocus />
        </label>
        {st.message && !error && st.captcha_rev && st.captcha_rev > 1 && <p className="hint">{st.message}</p>}
        {error && <p className="field-error" role="alert">{error}</p>}
        <div className="card-actions">
          <button className="btn btn-primary" disabled={busy || !code.trim()}>{busy ? '正在登录…' : '继续登录'}</button>
          <button type="button" className="btn btn-quiet" onClick={restart}>换个账号</button>
        </div>
      </form>
    );
  }

  if (st?.state === 'need_verify') {
    const what = st.verify_type === 'email' ? '邮箱' : '手机';
    return (
      <form className="form pw" onSubmit={(e) => { e.preventDefault(); run(() => api.submitVerify(code.trim()), true); }}>
        <p className="pw-step">密码正确。小米要验证 <b>{st.account}</b> 绑定的{what}，这是它的安全策略，不是出错了。</p>
        <ol className="qr-steps">
          <li>点下面的按钮，在新窗口打开小米的验证页面</li>
          <li>在那个页面点 <b>获取验证码</b>，小米会发到你的{what}</li>
          <li>把收到的验证码填到这里。<b>不要在小米页面里提交</b>，否则这个码会被那边用掉</li>
        </ol>
        <p><a className="btn" href={st.verify_url} target="_blank" rel="noreferrer noopener">打开小米验证页面 ↗</a></p>
        <label className="field">
          <span>{what}验证码</span>
          <input value={code} onChange={(e) => setCode(e.target.value)} inputMode="numeric" autoComplete="one-time-code" required />
        </label>
        {error && <p className="field-error" role="alert">{error}</p>}
        <div className="card-actions">
          <button className="btn btn-primary" disabled={busy || !code.trim()}>{busy ? '正在验证…' : '完成登录'}</button>
          <button type="button" className="btn btn-quiet" onClick={restart}>重新开始</button>
        </div>
      </form>
    );
  }

  return (
    <form className="form pw" onSubmit={start}>
      <label className="field">
        <span>小米账号</span>
        <input value={account} onChange={(e) => setAccount(e.target.value)} placeholder="手机号、邮箱或小米 ID" autoComplete="off" autoCapitalize="off" spellCheck={false} required />
      </label>
      <label className="field">
        <span>密码</span>
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="off" required />
      </label>
      {error && <p className="field-error" role="alert">{error}</p>}
      <div className="card-actions">
        <button className="btn btn-primary" disabled={busy || !account.trim() || !password}>{busy ? <><span className="spinner spinner-tiny" />正在登录…</> : '登录'}</button>
      </div>
      <p className="hint">密码只用这一次，不保存。连续输错 5 次会暂停 15 分钟，避免小米锁定账号。</p>
    </form>
  );
}
