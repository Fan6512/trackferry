import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { api, ApiError, type Album, type Artist, type Device, type FeiniuInfo, type LibraryStatus, type Missing, type Shelf, type Track } from '../api';
import { Cover } from '../components/Cover';
import { Heart } from '../components/Heart';
import { Notice } from '../components/Notice';
import { Play } from '../components/Play';
import { day, length, singer } from '../copy';
import type { Place } from '../nav';
import { songPicture, useBackdrop } from '../stage/Stage';

type Nav = (p: Place, from?: Element | null) => void;

export function Library({ place, devices, navigate }: { place: Place; devices: Device[]; navigate: Nav }) {
  if (place.kind === 'album') return <AlbumPage key={place.id} id={place.id!} devices={devices} navigate={navigate} />;
  if (place.kind === 'artist') return <ArtistPage key={place.id} name={place.id!} devices={devices} navigate={navigate} />;
  if (place.kind === 'missing') return <MissingPage navigate={navigate} />;
  return <Front devices={devices} navigate={navigate} />;
}

// Coming back from an album or a singer should find the same page; a new visit gets a new draw.
let keptShelf: Shelf | null = null;
let keepNext = false;
let keptQuery = '';
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
            <Play trackID={t.id} title={t.title} devices={devices} />
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

function Front({ devices, navigate }: { devices: Device[]; navigate: Nav }) {
  const [shelf, setShelf] = useState<Shelf | null>(() => (keepNext ? keptShelf : null));
  const fresh = useRef(!keepNext);
  const [drawing, setDrawing] = useState(false);
  useEffect(() => { keepNext = false; }, []);
  const [lib, setLib] = useState<LibraryStatus | null>(null);
  const [fn, setFn] = useState<FeiniuInfo | null>(null);
  const [q, setQ] = useState(keptQuery);
  const [found, setFound] = useState<Track[] | null>(null);
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
        <label className="lib-search">
          <svg viewBox="0 0 20 20" width="20" height="20" aria-hidden="true"><circle cx="8.5" cy="8.5" r="6" fill="none" stroke="currentColor" strokeWidth="2" /><path d="M13 13l5 5" stroke="currentColor" strokeWidth="2" /></svg>
          <input type="search" value={q} onChange={(e) => setQ(e.target.value)} placeholder="歌名、歌手、专辑" aria-label="搜索曲库" maxLength={100} />
        </label>
      </header>
      {error && <Notice tone="error" title={error} />}
      {(!fn.feiniu.linked || fn.feiniu.expired) && (
        <Notice tone={fn.feiniu.expired ? 'error' : 'warn'} title={fn.feiniu.expired ? '飞牛音乐的登录已失效' : '还没有连接飞牛音乐'}>
          <div className="notice-actions"><button className="btn btn-small" onClick={() => navigate({ page: 'account' })}>{fn.feiniu.expired ? '重新登录' : '去连接'}</button></div>
        </Notice>
      )}

      {found ? (
        <section className="found">
          {found.length === 0 ? <p className="muted">没有找到“{q.trim()}”。</p> : <Tracks tracks={found} devices={devices} showAlbum navigate={navigate} />}
          {found.length === 50 && <p className="muted center">只列前 50 首，写详细些。</p>}
        </section>
      ) : (
        <>
          {shelf?.wall && shelf.wall.length > 11 && <Wall albums={shelf.wall} navigate={navigate} />}
          {shelf && <div className="redraw"><button className="btn" disabled={drawing} onClick={redraw}>{drawing ? <span className="spinner spinner-tiny" /> : <svg viewBox="0 0 18 18" width="16" height="16" aria-hidden="true"><path d="M15.5 9a6.5 6.5 0 1 1-2-4.7M15.5 1.5v4h-4" fill="none" stroke="currentColor" strokeWidth="1.6" /></svg>}换一批</button></div>}
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
          {shelf?.favorites && shelf.favorites.length > 0 && (
            <section className="shelf"><h2>收藏的</h2><Tracks key={shelf.favorites[0].id} tracks={shelf.favorites} devices={devices} showAlbum navigate={navigate} /></section>
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
        <p className="album-by"><a href={`#/library/artist/${encodeURIComponent(album.artist)}`} onClick={(e) => { e.preventDefault(); navigate({ page: 'library', kind: 'artist', id: album.artist }); }}>{album.artist}</a><span className="num">{album.tracks} 首{total > 0 && `，${Math.round(total / 60000)} 分钟`}</span></p>
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
        <div><h1>{name}</h1>{tracks && <p className="album-by"><span className="num">{tracks.length.toLocaleString('zh-CN')} 首，{list.length} 张专辑</span></p>}</div>
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
