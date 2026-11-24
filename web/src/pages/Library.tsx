import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { api, ApiError, type Album, type Artist, type Device, type FeiniuInfo, type LibraryStatus, type Missing, type OnlineOutcome, type OnlineSong, type Playlist, type Shelf, type Track } from '../api';
import { Cover } from '../components/Cover';
import { Heart } from '../components/Heart';
import { Notice } from '../components/Notice';
import { Play } from '../components/Play';
import { SourceIcon } from '../components/SourceIcon';
import { stopLabel } from '../route';
import { day, length, singer } from '../copy';
import { href, type Place } from '../nav';
import { songPicture, useBackdrop } from '../stage/Stage';

type Nav = (p: Place, from?: Element | null) => void;

export function Library({ place, devices, navigate }: { place: Place; devices: Device[]; navigate: Nav }) {
  if (place.kind === 'album') return <AlbumPage key={place.id} id={place.id!} devices={devices} navigate={navigate} />;
  if (place.kind === 'artist') return <ArtistPage key={place.id} name={place.id!} devices={devices} navigate={navigate} />;
  if (place.kind === 'favorites') return <FavoritesPage devices={devices} navigate={navigate} />;
  if (place.kind === 'playlist') return <PlaylistPage key={place.id} guid={place.id!} devices={devices} navigate={navigate} />;
  if (place.kind === 'missing') return <MissingPage navigate={navigate} />;
  return <Front devices={devices} navigate={navigate} />;
}

// Coming back from an album or a singer should find the same page; a new visit gets a new draw.
let keptShelf: Shelf | null = null;
let keepNext = false;
let keptQuery = '';
let keptAsked = '';
const keptAlbums = new Map<string, Album>();
const keep = (as: Album[] | null | undefined) => as?.forEach((a) => keptAlbums.set(a.id, a));

function open(a: Album, e: React.MouseEvent, navigate: Nav) {
  keptAlbums.set(a.id, a);
  e.preventDefault();
  navigate({ page: 'library', kind: 'album', id: a.id }, e.currentTarget.querySelector('img'));
}

function Tile({ a, navigate, big = false }: { a: Album; navigate: Nav; big?: boolean }) {
  return (
    <a className="tile" href={`#/library/album/${a.id}`} onClick={(e) => open(a, e, navigate)}>
      <Cover cover={a.cover} title={a.name} artist={a.artist} size={big ? 1024 : 600} />
      <b>{a.name}</b><small>{a.artist}</small>
    </a>
  );
}

function Wall({ albums, navigate }: { albums: Album[]; navigate: Nav }) {
  const rows = [0, 1, 2].map((r) => albums.filter((_, i) => i % 3 === r));
  return (
    <div className="wall">
      {rows.map((row, r) => (
        <div key={r} className="wall-row" style={{ '--len': row.length, '--dir': r % 2 ? 'reverse' : 'normal' } as CSSProperties}>
          {[...row, ...row].map((a, i) => (
            <a key={i} href={`#/library/album/${a.id}`} tabIndex={i < row.length ? 0 : -1} aria-hidden={i >= row.length || undefined} aria-label={`${a.name}，${a.artist}`} onClick={(e) => open(a, e, navigate)}>
              <Cover cover={a.cover} title={a.name} artist={a.artist} size={600} />
              <span><b>{a.name}</b><small>{a.artist}</small></span>
            </a>
          ))}
        </div>
      ))}
    </div>
  );
}

function Tracks({ tracks, devices, numbered = false, showAlbum = false, navigate }: { tracks: Track[]; devices: Device[]; numbered?: boolean; showAlbum?: boolean; navigate: Nav }) {
  return (
    <ol className="tracks">
      {tracks.map((t, i) => (
        <li key={t.id}>
          <span className="track-lead">
            {numbered ? <span className="num track-no">{i + 1}</span> : <Cover cover={t.cover} title={t.title} artist={t.artist} />}
            <Play send={(d) => api.playTrack(d, t.id)} title={t.title} devices={devices} />
          </span>
          <span className="track-name">
            <b>{t.title}</b>
            <small>
              {singer(t.artist) ? <a href={`#/library/artist/${encodeURIComponent(t.artist)}`} onClick={(e) => { e.preventDefault(); navigate({ page: 'library', kind: 'artist', id: t.artist }); }}>{t.artist}</a> : '没有歌手信息'}
              {showAlbum && t.album && (t.album_id ? <a href={`#/library/album/${t.album_id}`} onClick={(e) => { e.preventDefault(); navigate({ page: 'library', kind: 'album', id: t.album_id }); }}>{t.album}</a> : <span>{t.album}</span>)}
            </small>
          </span>
          <span className="track-tech num">{t.format.toUpperCase()}{t.bits > 16 && t.format !== 'mp3' ? ` ${t.bits}bit` : ''}{t.sample_rate > 48000 ? ` ${t.sample_rate / 1000}k` : ''}</span>
          <Heart trackID={t.id} title={t.title} on={!!t.favorite} />
          <span className="num track-len">{length(t.duration_ms)}</span>
        </li>
      ))}
    </ol>
  );
}

const ONLINE = 'trackferry.search.online';

type OnlineFound = { songs: OnlineSong[]; sources: OnlineOutcome[] };
const isDown = (s: OnlineOutcome) => s.state === 'error' || s.state === 'open';
const ONLINE_KEPT = 5 * 60_000;
const keptOnline = new Map<string, { at: number; res: OnlineFound }>();
/** What the online sources answered a short while ago. */
function answered(q: string) {
  const k = keptOnline.get(q);
  return k && Date.now() - k.at < ONLINE_KEPT ? k.res : null;
}

/** Songs the online sources know for q. They are asked only when the listener says so: asked is
    what for, and a higher turn asks again. */
function Online({ q, asked, turn, ask, devices }: { q: string; asked: string; turn: number; ask: () => void; devices: Device[] }) {
  const [res, setRes] = useState<OnlineFound | null>(() => answered(q));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => {
    let on = true;
    const had = answered(q);
    setRes(had);
    setError(null);
    setBusy(false);
    if (had || asked !== q) return;
    setBusy(true);
    api.searchOnline(q).then((r) => {
      if (!r.sources.some(isDown)) {
        if (keptOnline.size >= 30) keptOnline.delete(keptOnline.keys().next().value!);
        keptOnline.set(q, { at: Date.now(), res: r });
      }
      if (on) { setRes(r); setBusy(false); }
    }, (e) => { if (on) { setError((e as ApiError).message); setBusy(false); } });
    return () => { on = false; };
  }, [q, asked, turn]);
  const down = res?.sources.filter(isDown) ?? [];
  return (
    <section className="shelf online">
      <h2>在线源{busy && <span className="spinner spinner-tiny" />}</h2>
      {!res && !busy && <button type="button" className="btn btn-quiet online-ask" onClick={ask}>{error ? '再搜一次' : `在线搜“${q}”`}</button>}
      {error && <p className="field-error" role="alert">{error}</p>}
      {res && !busy && res.songs.length === 0 && <p className="muted">在线源也没有找到。</p>}
      {res && res.songs.length > 0 && (
        <ol className="tracks online-list">
          {res.songs.map((s) => (
            <li key={s.key} className={s.restricted ? 'locked' : ''}>
              <span className="track-lead"><SourceIcon name={s.source} />{!s.restricted && <Play send={(d) => api.play(d, { source: 'online', id: s.key })} title={s.title} devices={devices} />}</span>
              <span className="track-name"><b>{s.title}</b><small><span>{s.artist || '没有歌手信息'}</span><span>{stopLabel[s.source]}</span></small></span>
              <span className="online-note">{s.restricted ? '要会员' : s.note}</span>
              <span className="num track-len">{s.duration_ms ? length(s.duration_ms) : ''}</span>
            </li>
          ))}
        </ol>
      )}
      {!busy && down.length > 0 && <p className="muted">{down.map((s) => stopLabel[s.source as OnlineSong['source']]).join('、')}这次没有回应。<button type="button" className="btn btn-quiet btn-small" onClick={ask}>再搜一次</button></p>}
    </section>
  );
}

const more = (p: Place, text: string, navigate: Nav) => <a className="shelf-more" href={href(p)} onClick={(e) => { e.preventDefault(); navigate(p); }}>{text}</a>;

/** Rows of favorites on the front. */
const FEW = 6;

function Favorites({ devices, navigate }: { devices: Device[]; navigate: Nav }) {
  const [fav, setFav] = useState<{ tracks: Track[] | null; total: number } | null>(null);
  useEffect(() => { api.favorites().then(setFav).catch(() => {}); }, []);
  if (!fav?.total) return null;
  return (
    <section className="shelf">
      <header className="shelf-head"><h2>收藏的<small className="num">{fav.total}</small></h2><Play send={(d) => api.play(d, { source: 'favorites' })} title="收藏" devices={devices} label="播放收藏" />{fav.total > FEW && more({ page: 'library', kind: 'favorites' }, '看全部', navigate)}</header>
      <Tracks tracks={(fav.tracks ?? []).slice(0, FEW)} devices={devices} showAlbum navigate={navigate} />
    </section>
  );
}

/** Playlists shown before the rest is asked for. */
const LISTS = 12;

function Playlists({ devices, navigate }: { devices: Device[]; navigate: Nav }) {
  const [lists, setLists] = useState<Playlist[]>([]);
  const [all, setAll] = useState(false);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => { api.playlists().then((r) => setLists(r.playlists)).catch((e) => setError((e as ApiError).message)); }, []);
  if (!lists.length) return error ? <p className="muted">{error}</p> : null;
  return (
    <section className="shelf">
      <header className="shelf-head"><h2>歌单<small className="num">{lists.length}</small></h2>{lists.length > LISTS && <button type="button" className="shelf-more" onClick={() => setAll(!all)}>{all ? '收起' : '看全部'}</button>}</header>
      <ul className="lists">
        {(all ? lists : lists.slice(0, LISTS)).map((l) => (
          <li key={l.guid}>
            <a href={href({ page: 'library', kind: 'playlist', id: l.guid })} onClick={(e) => { e.preventDefault(); navigate({ page: 'library', kind: 'playlist', id: l.guid }); }}>{l.name}</a>
            <Play send={(d) => api.play(d, { source: 'playlist', id: l.guid })} title={`歌单${l.name}`} devices={devices} />
          </li>
        ))}
      </ul>
    </section>
  );
}

function FavoritesPage({ devices, navigate }: { devices: Device[]; navigate: Nav }) {
  useEffect(() => { keepNext = true; }, []);
  useBackdrop();
  const [tracks, setTracks] = useState<Track[] | null>(null);
  const [total, setTotal] = useState(0);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const load = async (from: number) => {
    setBusy(true);
    try {
      const r = await api.favorites(from);
      setTracks((cur) => { const had = from ? cur ?? [] : []; const ids = new Set(had.map((t) => t.id)); return [...had, ...(r.tracks ?? []).filter((t) => !ids.has(t.id))]; });
      setTotal(r.tracks?.length ? r.total : Math.min(r.total, from));
      setError(null);
    } catch (e) { setError((e as ApiError).message); } finally { setBusy(false); }
  };
  useEffect(() => { load(0); }, []);
  return (
    <article className="artist">
      <Back navigate={navigate} />
      <header className="list-head"><h1>收藏的</h1>{tracks && <span className="num">{total.toLocaleString('zh-CN')} 首</span>}{total > 0 && <Play send={(d) => api.play(d, { source: 'favorites' })} title="收藏" devices={devices} label="播放收藏" />}</header>
      {error && <Notice tone="error" title={error} />}
      {tracks ? <Tracks tracks={tracks} devices={devices} showAlbum navigate={navigate} /> : !error && <div className="boot"><span className="spinner" /></div>}
      {tracks && tracks.length < total && <p className="center"><button className="btn btn-quiet" disabled={busy} onClick={() => load(tracks.length)}>{busy ? '正在读' : '再看一些'}</button></p>}
      {tracks && total === 0 && <p className="muted">还没有收藏。</p>}
    </article>
  );
}

function PlaylistPage({ guid, devices, navigate }: { guid: string; devices: Device[]; navigate: Nav }) {
  useEffect(() => { keepNext = true; }, []);
  useBackdrop();
  const [list, setList] = useState<{ name: string; tracks: Track[]; total: number } | null>(null);
  const [shown, setShown] = useState(60);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => { api.playlist(guid).then(setList).catch((e) => setError((e as ApiError).message)); }, [guid]);
  return (
    <article className="artist">
      <Back navigate={navigate} />
      {error && <Notice tone="error" title={error} />}
      {!list && !error && <div className="boot"><span className="spinner" /></div>}
      {list && <>
        <header className="list-head"><h1>{list.name}</h1><span className="num">{list.total.toLocaleString('zh-CN')} 首</span>{list.total > 0 && <Play send={(d) => api.play(d, { source: 'playlist', id: guid })} title={`歌单${list.name}`} devices={devices} label="播放歌单" />}</header>
        <Tracks tracks={list.tracks.slice(0, shown)} devices={devices} showAlbum navigate={navigate} />
        {shown < list.tracks.length && <p className="center"><button className="btn btn-quiet" onClick={() => setShown(shown + 60)}>再看一些</button></p>}
        {shown >= list.tracks.length && list.total > list.tracks.length && <p className="muted center">只列前 {list.tracks.length} 首，播放时全都会放。</p>}
        {list.total === 0 && <p className="muted">这个歌单里没有曲库能放的歌。</p>}
      </>}
    </article>
  );
}

function Front({ devices, navigate }: { devices: Device[]; navigate: Nav }) {
  const [shelf, setShelf] = useState<Shelf | null>(() => (keepNext ? keptShelf : null));
  const fresh = useRef(!keepNext);
  const [drawing, setDrawing] = useState(false);
  useEffect(() => { keepNext = false; }, []);
  const [lib, setLib] = useState<LibraryStatus | null>(null);
  const [fn, setFn] = useState<FeiniuInfo | null>(null);
  const [q, setQ] = useState(keptQuery);
  const [found, setFound] = useState<Track[] | null>(null);
  const [online, setOnline] = useState(() => { try { return localStorage.getItem(ONLINE) === '1'; } catch { return false; } });
  const [asked, setAsked] = useState({ q: keptAsked, turn: 0 });
  const ask = () => {
    keptAsked = q.trim();
    setAsked((a) => ({ q: keptAsked, turn: a.turn + 1 }));
  };
  const [error, setError] = useState<string | null>(null);
  const seq = useRef(0);
  const lead = shelf?.newest?.find((a) => a.cover) ?? shelf?.wall?.[0];
  useBackdrop(lead ? songPicture({ title: lead.name, artist: lead.artist, cover: lead.cover }) : undefined);

  const [lack, setLack] = useState(0);
  useEffect(() => { api.missing().then((r) => setLack(r.songs.filter((s) => !s.owned).length)).catch(() => {}); }, []);

  const running = lib?.running ?? false;
  useEffect(() => {
    let on = true;
    const load = async () => {
      try {
        const [l, f] = await Promise.all([api.library(), api.feiniu()]);
        if (!on) return;
        setLib(l.library);
        setFn(f);
        if (!l.library.running) {
          const s = await api.shelf();
          if (!on) return;
          // Every draw differs; keep the one already on screen.
          if (fresh.current || !keptShelf || keptShelf.albums !== s.albums) {
            fresh.current = false; keptShelf = s; setShelf(s); }
          keep(s.newest); keep(s.wall);
        }
        setError(null);
      } catch (e) {
        if (on) setError((e as ApiError).message);
      }
    };
    load();
    const t = setInterval(() => { if (!document.hidden) load(); }, running ? 1500 : 30000);
    return () => { on = false; clearInterval(t); };
  }, [running]);

  useEffect(() => {
    keptQuery = q;
    const id = ++seq.current;
    if (!q.trim()) { setFound(null); return; }
    const t = setTimeout(async () => {
      try {
        const r = await api.searchLibrary(q.trim());
        if (id === seq.current) setFound(r.tracks ?? []);
      } catch (e) {
        if (id === seq.current) setError((e as ApiError).message);
      }
    }, 180);
    return () => clearTimeout(t);
  }, [q]);

  const redraw = async () => {
    setDrawing(true);
    try {
      const s = await api.shelf();
      keptShelf = s;
      setShelf(s);
      keep(s.newest); keep(s.wall);
      setError(null);
    } catch (e) {
      setError((e as ApiError).message);
    } finally {
      setDrawing(false);
    }
  };

  if (!lib || !fn) return error ? <Notice tone="error" title={error} /> : <div className="boot"><span className="spinner" /></div>;
  const n = (v: number) => v.toLocaleString('zh-CN');
  const artists = shelf?.artists ?? [];
  const newest = shelf?.newest ?? [];

  return (
    <div className="library">
      <header className="lib-head">
        <h1><span className="num">{n(running ? Math.max(lib.seen, lib.tracks) : lib.tracks)}</span> 首歌</h1>
        <p className="lib-sub">{running ? '正在从飞牛音乐同步' : <>{shelf && shelf.albums > 0 && <>{n(shelf.albums)} 张专辑，</>}{n(lib.artists)} 位歌手{fn.feiniu.linked && <>，来自飞牛音乐</>}</>}{lack > 0 && <a href="#/library/missing" onClick={(e) => { e.preventDefault(); navigate({ page: 'library', kind: 'missing' }); }}>缺 <span className="num">{n(lack)}</span> 首</a>}</p>
        <form className="lib-search" role="search" onSubmit={(e) => { e.preventDefault(); if (online) ask(); (document.activeElement as HTMLElement | null)?.blur(); }}>
        <label>
          <svg viewBox="0 0 20 20" width="20" height="20" aria-hidden="true"><circle cx="8.5" cy="8.5" r="6" fill="none" stroke="currentColor" strokeWidth="2" /><path d="M13 13l5 5" stroke="currentColor" strokeWidth="2" /></svg>
          <input type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder={online ? '回车搜在线源' : '歌名、歌手、专辑'} enterKeyHint="search" aria-label="搜索曲库" maxLength={100} />
        </label>
        <label className="lib-online"><input type="checkbox" checked={online} onChange={(e) => { setOnline(e.target.checked); if (e.target.checked && q.trim()) ask(); try { localStorage.setItem(ONLINE, e.target.checked ? '1' : ''); } catch { /* for this visit only */ } }} />在线源</label>
        </form>
      </header>
      {error && <Notice tone="error" title={error} />}
      {(!fn.feiniu.linked || fn.feiniu.expired) && (
        <Notice tone={fn.feiniu.expired ? 'error' : 'warn'} title={fn.feiniu.expired ? '飞牛音乐的登录已失效' : '还没有连接飞牛音乐'}>
          <div className="notice-actions"><button className="btn btn-small" onClick={() => navigate({ page: 'account' })}>{fn.feiniu.expired ? '重新登录' : '去连接'}</button></div>
        </Notice>
      )}

      {found ? (
        <section className="found">
          {found.length === 0 ? <p className="muted">曲库里没有“{q.trim()}”。</p> : <Tracks tracks={found} devices={devices} showAlbum navigate={navigate} />}
          {found.length === 50 && <p className="muted center">只列前 50 首，写详细些。</p>}
          {online && q.trim() && <Online q={q.trim()} asked={asked.q} turn={asked.turn} ask={ask} devices={devices} />}
        </section>
      ) : (
        <>
          {shelf?.wall && shelf.wall.length > 11 && <Wall albums={shelf.wall} navigate={navigate} />}
          {shelf && <div className="redraw"><button className="btn" disabled={drawing} onClick={redraw}>{drawing ? <span className="spinner spinner-tiny" /> : <svg viewBox="0 0 18 18" width="16" height="16" aria-hidden="true"><path d="M15.5 9a6.5 6.5 0 1 1-2-4.7M15.5 1.5v4h-4" fill="none" stroke="currentColor" strokeWidth="1.6" /></svg>}换一批</button></div>}
          <Favorites devices={devices} navigate={navigate} />
          <Playlists devices={devices} navigate={navigate} />
          {shelf?.picks && shelf.picks.length > 0 && (
            <section className="shelf"><h2>随手翻到的</h2><Tracks key={shelf.picks[0].id} tracks={shelf.picks} devices={devices} showAlbum navigate={navigate} /></section>
          )}
          {newest.length > 0 && (
            <section className="shelf">
              <h2>新到的专辑</h2>
              <div className="newest">{newest.slice(0, 15).map((a, i) => <Tile key={a.id} a={a} navigate={navigate} big={i === 0} />)}</div>
            </section>
          )}
          {artists.length > 0 && (
            <section className="shelf">
              <h2>歌手</h2>
              <ul className="artists">
                {artists.map((a: Artist) => (
                  <li key={a.name}><a href={`#/library/artist/${encodeURIComponent(a.name)}`} onClick={(e) => { e.preventDefault(); navigate({ page: 'library', kind: 'artist', id: a.name }, e.currentTarget.querySelector('img')); }}>
                    <Cover cover={a.cover} title={a.name} /><b>{a.name}</b><small className="num">{n(a.tracks)}</small>
                  </a></li>
                ))}
              </ul>
            </section>
          )}
        </>
      )}
    </div>
  );
}

function Back({ navigate }: { navigate: Nav }) {
  return <button className="back" onClick={() => (history.length > 1 ? history.back() : navigate({ page: 'library' }))}><svg viewBox="0 0 18 14" width="18" height="14" aria-hidden="true"><path d="M17 7H2M8 1L2 7l6 6" fill="none" stroke="currentColor" strokeWidth="2" /></svg>曲库</button>;
}

function AlbumPage({ id, devices, navigate }: { id: string; devices: Device[]; navigate: Nav }) {
  useEffect(() => { keepNext = true; }, []);
  const [album, setAlbum] = useState<Album | null>(keptAlbums.get(id) ?? null);
  const [tracks, setTracks] = useState<Track[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  useBackdrop(album ? songPicture({ title: album.name, artist: album.artist, cover: album.cover }) : undefined);
  useEffect(() => {
    api.album(id).then((r) => { setAlbum(r.album); setTracks(r.tracks); }).catch((e) => setError((e as ApiError).message));
  }, [id]);
  if (!album) return <>{error ? <><Back navigate={navigate} /><Notice tone="error" title={error} /></> : <div className="boot"><span className="spinner" /></div>}</>;
  const total = tracks?.reduce((a, t) => a + t.duration_ms, 0) ?? 0;
  return (
    <article className="album">
      <div className="album-side">
        <Back navigate={navigate} />
        <Cover className="hero" cover={album.cover} title={album.name} artist={album.artist} size={1024} eager />
      </div>
      <div className="album-main">
        <h1>{album.name}</h1>
        <p className="album-by"><a href={`#/library/artist/${encodeURIComponent(album.artist)}`} onClick={(e) => { e.preventDefault(); navigate({ page: 'library', kind: 'artist', id: album.artist }); }}>{album.artist}</a><span className="num">{album.tracks} 首{total > 0 && `，${Math.round(total / 60000)} 分钟`}</span><Play send={(d) => api.play(d, { source: 'album', id })} title={album.name} devices={devices} label="播放专辑" /></p>
        {error && <Notice tone="error" title={error} />}
        {tracks && <Tracks tracks={tracks} devices={devices} numbered navigate={navigate} />}
      </div>
    </article>
  );
}

function ArtistPage({ name, devices, navigate }: { name: string; devices: Device[]; navigate: Nav }) {
  useEffect(() => { keepNext = true; }, []);
  const [tracks, setTracks] = useState<Track[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [all, setAll] = useState(false);
  useEffect(() => {
    api.artist(name).then((r) => setTracks(r.tracks ?? [])).catch((e) => setError((e as ApiError).message));
  }, [name]);
  const albums = new Map<string, Album>();
  for (const t of tracks ?? []) {
    if (!t.album_id || !t.album) continue;
    const a = albums.get(t.album_id) ?? { id: t.album_id, name: t.album, artist: t.artist, cover: '', tracks: 0, added_ms: 0 };
    a.tracks++;
    a.cover ||= t.cover ?? '';
    albums.set(t.album_id, a);
  }
  const list = [...albums.values()].sort((a, b) => b.tracks - a.tracks);
  const lead = list.find((a) => a.cover);
  useBackdrop(tracks ? songPicture({ title: name, cover: lead?.cover }) : undefined);
  return (
    <article className="artist">
      <Back navigate={navigate} />
      <header className="artist-head">
        <Cover key={lead?.cover} className="hero" cover={lead?.cover} title={name} size={600} eager />
        <div><h1>{name}</h1>{tracks && <p className="album-by"><span className="num">{tracks.length.toLocaleString('zh-CN')} 首，{list.length} 张专辑</span>{tracks.length > 0 && <Play send={(d) => api.play(d, { source: 'artist', name })} title={`${name}的歌`} devices={devices} label="播放这位歌手" />}</p>}</div>
      </header>
      {error && <Notice tone="error" title={error} />}
      {list.length > 0 && <section className="shelf"><h2>专辑</h2><div className="rail">{list.map((a) => <Tile key={a.id} a={a} navigate={navigate} />)}</div></section>}
      {tracks && tracks.length > 0 && (
        <section className="shelf"><h2>歌曲</h2><Tracks tracks={all ? tracks : tracks.slice(0, 60)} devices={devices} showAlbum navigate={navigate} />
          {!all && tracks.length > 60 && <p className="center"><button className="btn btn-quiet" onClick={() => setAll(true)}>显示全部 {tracks.length} 首</button></p>}
        </section>
      )}
      {tracks && tracks.length === 0 && <p className="muted">曲库里没有这位歌手的歌。</p>}
    </article>
  );
}

function MissingPage({ navigate }: { navigate: Nav }) {
  useEffect(() => { keepNext = true; }, []);
  const [songs, setSongs] = useState<Missing[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  useEffect(() => { api.missing().then((r) => setSongs(r.songs)).catch((e) => setError((e as ApiError).message)); }, []);
  const lack = songs?.filter((s) => !s.owned) ?? [];
  const owned = songs?.filter((s) => s.owned) ?? [];
  const list = (of: Missing[]) => (
    <ol className="tracks lacking">
      {of.map((s) => (
        <li key={`${s.title} ${s.artist} ${s.last_ms}`}>
          <span className="track-name"><b>{s.title}</b><small>{s.artist || '没有歌手信息'}</small></span>
          <span className="track-tech">{day(s.last_ms)}</span>
          <span className="track-len"><span className="num">{s.plays}</span> 次</span>
        </li>
      ))}
    </ol>
  );
  return (
    <article className="artist">
      <Back navigate={navigate} />
      <header className="lack-head">
        <div><h1>缺歌清单</h1><p>近 180 天从线上放过、曲库里还没有的歌。补进飞牛音乐并同步后，会自动划掉。</p></div>
        {lack.length > 0 && <a className="btn" href="/api/library/missing.csv" download>导出表格</a>}
      </header>
      {error && <Notice tone="error" title={error} />}
      {!songs && !error && <div className="boot"><span className="spinner" /></div>}
      {songs && lack.length === 0 && <p className="muted">没有缺的歌。</p>}
      {lack.length > 0 && list(lack)}
      {owned.length > 0 && <section className="shelf"><h2>已经补上的</h2><div className="lack-owned">{list(owned)}</div></section>}
    </article>
  );
}
