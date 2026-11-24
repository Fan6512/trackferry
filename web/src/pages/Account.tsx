import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { api, ApiError, type Account as AccountT, type LinkResult, type UpdateStatus } from '../api';
import { Notice } from '../components/Notice';
import { FeiniuCard } from '../components/Feiniu';
import { LockIcon, Setting, type Light } from '../components/Setting';
import { SourceIcon } from '../components/SourceIcon';
import { Sources } from '../components/Sources';
import { useBackdrop } from '../stage/Stage';
import { LinkAccount } from '../components/LinkAccount';
import { explainSync, timeAgo } from '../copy';
import fish from '../assets/brand/fish-mark.webp';
import word from '../assets/brand/wordmark.svg';
import './settings.css';

const parts = [['feiniu', '飞牛音乐'], ['xiaomi', '小米账号'], ['sources', '线上音源'], ['password', '管理密码'], ['about', '关于']] as const;
type Part = (typeof parts)[number][0];

export function Account({ account, build, startScanning, onChanged, onLogout }: {
  account: AccountT;
  build: { version: string; commit: string };
  startScanning: boolean;
  onChanged: () => Promise<void>;
  onLogout: () => void;
}) {
  useBackdrop();
  const [scanning, setScanning] = useState(startScanning);
  const [linkedNow, setLinkedNow] = useState<LinkResult | null>(null);
  const [confirmUnlink, setConfirmUnlink] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const onLinked = async (qr: LinkResult) => {
    setLinkedNow(qr);
    setScanning(false);
    await onChanged();
  };

  const unlink = async () => {
    setBusy(true);
    setError(null);
    try {
      await api.unlink();
      setConfirmUnlink(false);
      setLinkedNow(null);
      await onChanged();
    } catch (e) {
      setError((e as ApiError).message);
    } finally {
      setBusy(false);
    }
  };

  const [lights, setLights] = useState<Partial<Record<Part, Light>>>({});
  const feiniuLight = useCallback((l: Light) => setLights((x) => ({ ...x, feiniu: l })), []);
  const sourceLight = useCallback((l: Light) => setLights((x) => ({ ...x, sources: l })), []);
  const aboutLight = useCallback((l: Light | null) => setLights((x) => ({ ...x, about: l ?? undefined })), []);
  const mine: Light = !account.linked ? 'warn' : account.relink_required ? 'bad' : 'ok';
  const [at, setAt] = useState<Part>('feiniu');
  useEffect(() => {
    const f = () => {
      const tops = parts.map(([p]) => [p, document.getElementById(`set-${p}`)?.getBoundingClientRect().top ?? 1e6] as const);
      const end = innerHeight + scrollY >= document.documentElement.scrollHeight - 4 && scrollY > 0;
      setAt(scrollY < 8 ? 'feiniu' : end ? 'about' : (tops.filter(([, y]) => y < innerHeight * .35).at(-1)?.[0] ?? 'feiniu'));
    };
    f();
    addEventListener('scroll', f, { passive: true });
    return () => removeEventListener('scroll', f);
  }, []);
  const go = (p: Part) => document.getElementById(`set-${p}`)?.scrollIntoView({ behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth', block: 'start' });

  const explanation = linkedNow?.sync ? explainSync(linkedNow.sync) : null;

  return (
    <div className="settings">
      <header className="page-head"><h1>设置</h1></header>
      <div className="st-cols">
      <nav className="st-nav" aria-label="设置分类">
        {parts.map(([p, name]) => { const l = p === 'xiaomi' ? mine : lights[p]; return <button key={p} type="button" aria-current={at === p || undefined} onClick={() => go(p)}>{name}{l && <i className={l} />}</button>; })}
      </nav>
      <div className="st-list">
      {explanation && <Notice tone={explanation.tone} title={`已关联新的登录，${explanation.title}`}>{explanation.body && <p>{explanation.body}</p>}</Notice>}

      <FeiniuCard onLight={feiniuLight} />

      <Setting id="xiaomi" icon={<SourceIcon name="xiaomi" />} title="小米账号" light={mine} state={account.linked ? (account.relink_required ? '登录已失效' : '已关联') : '未关联'}
        says={account.last_sync && account.linked ? explainSync(account.last_sync).title : '用来读取音箱听到的话和控制播放'}
        actions={scanning ? undefined : <><button className={`btn ${account.linked && !account.relink_required ? '' : 'btn-primary'}`} onClick={() => setScanning(true)}>{account.linked ? '重新登录' : '关联小米账号'}</button>{account.linked && !confirmUnlink && <button className="btn btn-quiet" onClick={() => setConfirmUnlink(true)}>解除关联</button>}</>}>
          {account.linked && <dl className="kv">
            {account.user_id && <div><dt>小米 ID</dt><dd className="mono">{account.user_id}</dd></div>}
            {account.linked_at && <div><dt>关联</dt><dd>{timeAgo(account.linked_at)}</dd></div>}
            {account.token_obtained_at && <div><dt>访问令牌</dt><dd>{timeAgo(account.token_obtained_at)}更新，到期前自动续期</dd></div>}
            {account.last_sync && <div><dt>读取音箱</dt><dd>{timeAgo(account.last_sync.at)}</dd></div>}
          </dl>}
          {confirmUnlink && (
            <div className="confirm">
              <p>解除后会删除 NAS 上的小米登录凭据，不再读取口令。音箱设置会保留，下次关联后恢复。</p>
              <div className="card-actions">
                <button className="btn btn-danger" onClick={unlink} disabled={busy}>{busy ? '正在解除…' : '确认解除'}</button>
                <button className="btn btn-quiet" onClick={() => setConfirmUnlink(false)}>取消</button>
              </div>
            </div>
          )}
          {error && <p className="field-error" role="alert">{error}</p>}
          {scanning && (
            <div className="linking">
              {account.linked && <p className="hint">登录成功后替换现在的登录。<button className="link" onClick={() => { api.cancelQR().catch(() => {}); api.cancelPassword().catch(() => {}); setScanning(false); }}>取消</button></p>}
              <LinkAccount onLinked={onLinked} />
            </div>
          )}
      </Setting>

      <Sources onLight={sourceLight} />
      <PasswordCard onLogout={onLogout} />
      <About build={build} onLight={aboutLight} />
      </div>
      </div>
    </div>
  );
}

function PasswordCard({ onLogout }: { onLogout: () => void }) {
  const [open, setOpen] = useState(false);
  const [cur, setCur] = useState('');
  const [next, setNext] = useState('');
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setMsg(null);
    try {
      await api.changePassword(cur, next);
      setMsg({ ok: true, text: '已修改。其他浏览器已退出登录。' });
      setCur('');
      setNext('');
      setOpen(false);
    } catch (err) {
      setMsg({ ok: false, text: (err as ApiError).message });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Setting id="password" icon={<LockIcon />} title="管理密码" says="修改后其他浏览器会退出登录"
      actions={<>{!open && <button className="btn" onClick={() => setOpen(true)}>修改密码</button>}<button className="btn btn-quiet" onClick={onLogout}>退出登录</button></>}>
      {msg && <p className={msg.ok ? 'ok-text' : 'field-error'}>{msg.text}</p>}
      {open && (
        <form className="form form-inline" onSubmit={submit}>
          <label className="field"><span>当前密码</span><input type="password" value={cur} onChange={(e) => setCur(e.target.value)} autoComplete="current-password" required /></label>
          <label className="field"><span>新密码（至少 8 位）</span><input type="password" value={next} onChange={(e) => setNext(e.target.value)} autoComplete="new-password" minLength={8} required /></label>
          <div className="card-actions">
            <button className="btn btn-primary" disabled={busy}>{busy ? '正在保存…' : '保存'}</button>
            <button type="button" className="btn btn-quiet" onClick={() => setOpen(false)}>取消</button>
          </div>
        </form>
      )}
    </Setting>
  );
}

function About({ build, onLight }: { build: { version: string; commit: string }; onLight: (l: Light | null) => void }) {
  const [up, setUp] = useState<UpdateStatus | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => { api.update().then(setUp).catch(() => {}); }, []);
  useEffect(() => { onLight(up?.newer ? 'warn' : null); }, [up?.newer, onLight]);
  const check = async () => {
    setBusy(true);
    setError(null);
    try { setUp(await api.checkUpdate()); } catch (e) { setError((e as ApiError).message); } finally { setBusy(false); }
  };
  return (
    <section className="st" id="set-about">
      <div className="st-about">
        <img src={fish} alt="" width={96} height={96} />
        <div>
          <img src={word} alt="TrackFerry" width={150} />
          <p>小爱点歌，飞牛曲库优先</p>
          <small className="num">版本 {build.version}{build.commit && build.commit !== 'unknown' && `（${build.commit}）`}{up && !up.newer && up.checked_ms ? up.error ? `，${up.error}` : '，已是最新' : ''}</small>
        </div>
        <button className="btn" onClick={check} disabled={busy}>{busy ? <><span className="spinner spinner-tiny" />正在检查</> : '检查更新'}</button>
      </div>
      {error && <p className="field-error st-body">{error}</p>}
      {up?.newer && (
        <div className="st-body st-update">
          <h3><b className="num">{up.latest}</b>{up.published && <small>{new Date(up.published).toLocaleDateString('zh-CN')}发布</small>}{up.url && <a href={up.url} target="_blank" rel="noreferrer">更新说明</a>}</h3>
          {up.notes && <pre className="st-notes">{up.notes.replace(/^#+ +/gm, '').replace(/^[-*] +/gm, '· ')}</pre>}
          <div><p>把 compose.yaml 里的镜像标签改成 <span className="num">{up.latest}</span>，再执行下面的命令，数据不受影响：</p><pre>docker compose pull &amp;&amp; docker compose up -d</pre></div>
        </div>
      )}
    </section>
  );
}
