import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import { api, type Guide, type Example, type Reading, type Route, type Shelf, type Track } from '../api';
import type { Place } from '../nav';
import { Cover } from '../components/Cover';
import { SourceIcon } from '../components/SourceIcon';
import { fishPicture, songPicture, useWater } from '../stage/Stage';
import { Fish } from '../components/Fish';
import { stopLabel } from '../route';

type Part = { text: string; role?: string };
interface Way { key: string; name: string; parts: Part[]; note: string; pic?: { title: string; artist: string; cover?: string } }
interface Song { title: string; artist: string; cover?: string }

const said = (w: Way) => w.parts.map((p) => p.text).join('');
const plain: Song = { title: '晴天', artist: '周杰伦' };

/** A title someone would say aloud: short, no brackets, no version words. */
const sayable = (t: Track) => /^[\p{Script=Han}A-Za-z ]{2,8}$/u.test(t.title) && /^[\p{Script=Han}A-Za-z ]{2,12}$/u.test(t.artist);

/** Examples come from the listener's own library, so every one of them can really be said. */
function useSongs(): Song[] | null {
  const [songs, setSongs] = useState<Song[] | null>(null);
  useEffect(() => {
    let on = true;
    (async () => {
      const s: Shelf = await api.shelf();
      const names = (s.artists ?? []).map((a) => a.name).filter((n) => /^[\p{Script=Han}]{2,4}$/u.test(n)).slice(0, 4);
      const per = await Promise.all(names.map((n) => api.artist(n).then((r) => (r.tracks ?? []).filter((t) => t.artist === n && sayable(t)), () => [])));
      const fav = (s.favorites ?? []).filter(sayable);
      const out = [...fav.slice(0, 1), ...per.flatMap((l) => (l.find((t) => t.cover) ?? l[0] ? [l.find((t) => t.cover) ?? l[0]] : []))];
      if (on) setSongs(out.map((t) => ({ title: t.title, artist: t.artist, cover: t.cover })));
    })().catch(() => on && setSongs([]));
    return () => { on = false; };
  }, []);
  return songs;
}

function ways(songs: Song[]): Way[] {
  const at = (i: number) => songs[i % Math.max(1, songs.length)] ?? plain;
  const [a, b, c] = [at(0), at(1), at(2)];
  return [
    { key: 'full', name: '歌手和歌名', parts: [{ text: '播放', role: '口令' }, { text: a.artist, role: '歌手' }, { text: '的' }, { text: a.title, role: '歌名' }], note: '最准的说法，同名歌也不会放错', pic: a },
    { key: 'title', name: '只说歌名', parts: [{ text: '我想听', role: '口令' }, { text: b.title, role: '歌名' }], note: '好几位歌手都唱过时，先放公认的原唱；想听另一版就带上歌手', pic: b },
    { key: 'artist', name: '听一位歌手', parts: [{ text: '播放', role: '口令' }, { text: c.artist, role: '歌手' }, { text: '的歌' }], note: '曲库里这位歌手的歌，随机一首接一首。想只放一首，在“设置 → 连续播放”里关掉', pic: c },
    { key: 'any', name: '随便听听', parts: [{ text: '随便放', role: '口令' }, { text: '一首歌' }], note: '从曲库里随机挑一首。想一直放下去，在“设置 → 连续播放”里打开随机接着放' },
    { key: 'list', name: '连着听', parts: [{ text: '播放', role: '口令' }, { text: '我的收藏' }], note: '连着放飞牛音乐的收藏；说“播放某某歌单”就播那个歌单' },
    { key: 'control', name: '播放控制', parts: [{ text: '下一首', role: '口令' }], note: '暂停、继续由小爱自己做；连播时的上一首、下一首由我们换；说“停止”，连播就结束' },
  ];
}

const controlName: Record<string, string> = { pause: '暂停', resume: '继续', stop: '停止', next: '下一首', previous: '上一首' };

function verdict(r: Reading): { yes: boolean; text: string } {
  switch (r.why) {
    case 'song': return { yes: true, text: r.artist ? `点歌：${r.artist}的《${r.title}》` : `点歌：《${r.title}》` };
    case 'artist': return { yes: true, text: `点歌：${r.artist}的歌，随机一首接一首` };
    case 'random': return { yes: true, text: '点歌：随机一首' };
    case 'version': return { yes: true, text: r.artist ? `接着上一首：换成${r.artist}唱的${r.verb ? `；没点过歌就放${r.artist}的任意一首` : ''}` : '接着上一首：换一位歌手唱的' };
    case 'list': return { yes: true, text: r.control === 'favorites' ? '连着放飞牛音乐的收藏' : r.title ? `连着放飞牛音乐的歌单“${r.title}”` : '连着放飞牛音乐的歌单，有好几个时要说名字' };
    case 'favorite': if (r.control?.startsWith('list_')) return { yes: true, text: `放歌时说：${r.control === 'list_add' ? '加入' : '移出'}飞牛音乐的歌单${r.title ? `“${r.title}”` : ''}` };
      return { yes: true, text: r.control === 'remove' ? '放歌时说：移出飞牛音乐的收藏' : '放歌时说：加入飞牛音乐的收藏' };
    case 'control': return { yes: false, text: `播放控制：${controlName[r.control ?? ''] ?? '交给小爱'}，小爱自己做` };
    case 'not_music': return { yes: false, text: r.word ? `不是点歌：有“${r.word}”，交给小爱` : '不是点歌，交给小爱' };
    case 'no_verb': return { yes: false, text: '不是点歌：开头没有“播放”“我想听”这类词，交给小爱' };
    default: return { yes: false, text: '' };
  }
}

const groups: { key: string; name: string; pick: (x: Example) => boolean; no?: boolean }[] = [
  { key: 'song', name: '点一首', pick: (x) => x.kind === 'song' },
  { key: 'more', name: '歌手、随便听', pick: (x) => x.kind === 'artist' || x.kind === 'random' },
  { key: 'version', name: '换版本', pick: (x) => x.kind === 'version' },
  { key: 'list', name: '收藏和歌单连播', pick: (x) => x.kind === 'list' },
  { key: 'favorite', name: '加收藏、加歌单', pick: (x) => x.kind === 'favorite' },
  { key: 'no', name: '不算点歌', pick: () => true, no: true },
];

/** Say something; see at once what comes of it. */
function Talk({ guide }: { guide: Guide }) {
  const [text, setText] = useState('');
  const [r, setR] = useState<Reading | null>(null);
  const [g, setG] = useState('song');
  useEffect(() => {
    if (!text.trim()) { setR(null); return; }
    let on = true;
    const t = setTimeout(() => api.explain(text).then((x) => on && setR(x), () => on && setR(null)), 160);
    return () => { on = false; clearTimeout(t); };
  }, [text]);
  const group = groups.find((x) => x.key === g)!;
  const list = (group.no ? guide.no : guide.yes).filter(group.pick);
  const v = r && text.trim() ? verdict(r) : null;
  return (
    <section className="talk">
      <div className="talk-try">
        <label><span>小爱同学，</span><input value={text} onChange={(e) => setText(e.target.value)} maxLength={100} placeholder="写一句试试" aria-label="写一句，看算不算点歌" /></label>
        <p aria-live="polite" className={v ? (v.yes ? 'yes' : 'no') : ''}>{v ? <>{v.yes ? null : <SourceIcon name="xiaomi" />}{v.text}</> : '算不算点歌，当场告诉你'}</p>
      </div>
      <div className="talk-tabs" role="tablist" aria-label="例句">{groups.map((x) => <button key={x.key} type="button" role="tab" aria-selected={x.key === g} onClick={() => setG(x.key)}>{x.name}</button>)}</div>
      <ul key={g} className="talk-list">
        {list.slice(0, 9).map((x) => (
          <li key={x.say}>
            <button type="button" aria-pressed={text === x.say} onClick={() => setText(x.say)}><b>{x.say}</b><small>{group.no && <SourceIcon name="xiaomi" />}{x.then}</small></button>
          </li>
        ))}
      </ul>
    </section>
  );
}

/** The opening words, put in front of a real request. */
function Openers({ guide, song }: { guide: Guide; song: Song }) {
  const [v, setV] = useState('播放');
  const n = Math.max(0, guide.verbs.indexOf(v));
  const at = (l: string[]) => l[n % l.length];
  return (
    <section className="openers">
      <h2>用哪个词开头都行</h2>
      <p className="openers-line" aria-live="polite"><small>{at(guide.polite)}</small><em>{v}</em><span>{song.artist}</span><i>的</i><span>{song.title}</span><small>{at(guide.tails)}</small></p>
      <ul>{guide.verbs.map((x) => <li key={x}><button type="button" aria-pressed={x === v} onClick={() => setV(x)} onPointerEnter={(e) => { if (e.pointerType === 'mouse') setV(x); }}>{x}</button></li>)}</ul>
      <p className="openers-note">灰色的说不说都行</p>
    </section>
  );
}

interface Cut { key: string; say: string; who: string; after: 'same' | 'next' | 'stop' | 'out'; then: string }
const cuts: Cut[] = [
  { key: 'chat', say: '今天天气怎么样', who: '小爱回答', after: 'same', then: '从断开的地方接着放' },
  { key: 'fav', say: '收藏这首歌', who: '报一声结果', after: 'same', then: '压住小爱的回答，报完结果歌接着放。线上的歌不在曲库里，收藏不了' },
  { key: 'ver', say: '换个版本', who: '找另一位歌手', after: 'next', then: '同一首歌，换人唱。也可以说“放某某的”' },
  { key: 'song', say: '再点一首', who: '去找新歌', after: 'next', then: '换成新的这首' },
  { key: 'stop', say: '停止', who: '小爱停下', after: 'stop', then: '安静下来，连播也结束。只想停一会儿就说“暂停”，用“继续播放”回来' },
  { key: 'news', say: '播放新闻', who: '让给小爱', after: 'out', then: '音箱归小爱' },
];

const heart = 'M10 16.6S1.4 11.6 1.4 5.9C1.4 3.3 3.4 1.4 5.8 1.4c1.8 0 3.3 1 4.2 2.6.9-1.6 2.4-2.6 4.2-2.6 2.4 0 4.4 1.9 4.4 4.5 0 5.7-8.6 10.7-8.6 10.7z';
/** Loudness of bar i of a song: steady, so the same song looks the same before and after. */
const loud = (i: number, seed: number) => {
  const f = Math.sin(i * 0.23 + seed) * 0.3 + Math.sin(i * 0.71 + seed * 2.3) * 0.25 + Math.sin(i * 2.13 + seed * 0.7) * 0.22;
  return Math.max(0.12, 0.55 + f);
};
const BARS = 112, CUT = 40, BACK = 60;

/** What happens to a song when someone speaks over it: the sound itself, left to right. */
function Over({ songs }: { songs: Song[] }) {
  const [k, setK] = useState('chat');
  const c = cuts.find((x) => x.key === k)!;
  const a = songs[0] ?? plain, b = songs[1] ?? { title: '稻香', artist: '周杰伦' };
  const next = k === 'ver' ? { title: a.title, artist: b.artist, cover: b.cover } : b;
  const bars = Array.from({ length: BARS }, (_, i) => {
    if (i < CUT) return { h: loud(i, 1), w: 'a' };
    if (i < BACK) return { h: 0.07, w: 'q' };
    // Going on means going on from the bar it was cut at.
    if (c.after === 'same') return { h: loud(i - (BACK - CUT), 1), w: 'a' };
    if (c.after === 'next') return { h: loud(i, k === 'ver' ? 4.2 : 7.7), w: 'b' };
    if (c.after === 'out') return { h: 0.3 + 0.5 * Math.abs(Math.sin(i * 0.9) * Math.sin(i * 0.17)), w: 'x' };
    return { h: 0.07, w: 'q' };
  });
  return (
    <section className="over">
      <h2>放歌时开口</h2>
      <div className="over-in">
        <div className="over-says" role="tablist" aria-label="放歌时说的话">{cuts.map((x) => <button key={x.key} type="button" role="tab" aria-selected={x.key === k} onClick={() => setK(x.key)}>{x.say}</button>)}</div>
        <div className="over-scene">
          <div className="over-names">
            <span><Cover cover={a.cover} title={a.title} artist={a.artist} /><b>{a.title}</b></span>
            <span className="over-who">{c.who}</span>
            <span className="over-after" key={k}>
              {c.after === 'same' && (k === 'fav' ? <><svg viewBox="0 0 20 18" aria-hidden="true"><path d={heart} /></svg><b>已收藏</b></> : <b>接着放</b>)}
              {c.after === 'next' && <><Cover cover={next.cover} title={next.title} artist={next.artist} /><b>{next.title}</b><small>{next.artist}</small></>}
              {c.after === 'stop' && <b>安静</b>}
              {c.after === 'out' && <><SourceIcon name="xiaomi" /><b>小爱</b></>}
            </span>
          </div>
          <svg className="over-wave" viewBox={`0 0 ${BARS * 8} 120`} preserveAspectRatio="none" aria-hidden="true">
            {bars.map((x, i) => <rect key={i} className={`w-${x.w}`} x={i * 8 + 2} y="0" width="4" height="120" style={{ transform: `scaleY(${x.h})`, transitionDelay: i < CUT ? '0s' : `${(i - CUT) * 6}ms` }} />)}
          </svg>
          <p>{c.then}</p>
        </div>
      </div>
    </section>
  );
}

/** Where a song comes from, in the order it is looked for. */
function Where() {
  return (
    <section className="where">
      <h2>去哪里找</h2>
      <ol>
        <li><div className="where-icons"><SourceIcon name="library" /></div><b>你的曲库</b><p>先放你收藏的，再看原唱</p></li>
        <li><div className="where-icons"><SourceIcon name="kuwo" /><SourceIcon name="kugou" /><SourceIcon name="migu" /><SourceIcon name="netease" /></div><b>线上</b><p>曲库没有才问，只放免费完整版；登录网易云后，你账号能完整听的也放</p></li>
        <li><div className="where-icons"><SourceIcon name="xiaomi" /></div><b>小爱</b><p>都没找到，交给它</p></li>
      </ol>
    </section>
  );
}

/** What is started from a page, not by voice. */
function Taps({ navigate }: { navigate: (p: Place) => void }) {
  const list: { name: string; how: string; go: Place; to: string }[] = [
    { name: '把收藏、歌单送到音箱', how: '曲库页点收藏或歌单旁的播放键，选一台音箱。专辑和歌手页也有', go: { page: 'library' }, to: '去曲库' },
    { name: '点名字难念的歌', how: '搜索框旁勾上“在线源”，找到后点播放键。送过的照常缓存', go: { page: 'library' }, to: '去搜索' },
    { name: '一首放完接着放', how: '打开“点完一首接着随机放”，随后从飞牛曲库随机接歌', go: { page: 'account' }, to: '去设置' },
    { name: '到点就停', how: '“最长放多久”管所有连播，默认 3 小时，也可以不限', go: { page: 'account' }, to: '去设置' },
    { name: '压住小爱的回答', how: '接管的音箱可选普通、加强、等小爱说完。还听得到就往后换一档', go: { page: 'speakers' }, to: '去音箱' },
    { name: '暂停、停止、调音量', how: '“此刻”选中音箱就能按，音箱页也有', go: { page: 'home' }, to: '去此刻' },
  ];
  return (
    <section className="taps">
      <h2>不用说，点一下</h2>
      <ul>{list.map((x) => <li key={x.name}><button type="button" onClick={() => navigate(x.go)}><b>{x.name}</b><span>{x.how}</span><em>{x.to}</em></button></li>)}</ul>
    </section>
  );
}

const glyph: Record<string, string> = {
  previous: 'M6 5v14M20 5 9 12l11 7z', pause: 'M8 5v14M16 5v14', resume: 'M7 4.5 19.5 12 7 19.5z', stop: 'M6 6h12v12H6z', next: 'M18 5v14M4 5l11 7-11 7z',
  tell: 'M9 4.5a3 3 0 0 1 6 0v6a3 3 0 0 1-6 0zM5.5 10.5a6.5 6.5 0 0 0 13 0M12 17v4',
  sleep: 'M20 14.2A8.4 8.4 0 0 1 9.8 4 8.4 8.4 0 1 0 20 14.2z',
  learn: 'M12 6.5C10 5 7 4.6 3.5 5v13.5c3.5-.4 6.500 0 8.500 1.500 2-1.500 5-1.900 8.500-1.500V5C17 4.600 14 5 12 6.500zM12 6.500V20',
  watch: 'M3.500 5h17v11.500h-17zM8.500 20h7',
  ask: 'M6 17V11a6 6 0 0 1 12 0v6l1.500 1.500h-15zM10.300 21h3.400',
};
const Glyph = ({ name }: { name: string }) => <svg className="glyph" viewBox="0 0 24 24" aria-hidden="true"><path d={glyph[name]} /></svg>;

const shelves: { key: string; name: string; words: string[]; say: (w: string) => string }[] = [
  { key: 'tell', name: '听人讲', words: ['新闻', '故事', '相声', '评书', '小说', '电台', '广播', '有声书', '播客', '笑话', '节目'], say: (w) => `播放${w}` },
  { key: 'sleep', name: '哄睡', words: ['雨声', '海浪声', '白噪音', '睡前'], say: (w) => (w === '睡前' ? '放点睡前音乐' : `放一下${w}`) },
  { key: 'learn', name: '学东西', words: ['古诗', '课文', '英语', '单词'], say: (w) => `播放${w}` },
  { key: 'watch', name: '看屏幕', words: ['电影', '电视', '视频', '动画片'], say: (w) => `我要看${w}` },
  { key: 'ask', name: '问和提醒', words: ['天气', '闹钟'], say: (w) => (w === '天气' ? '今天天气怎么样' : '定个七点的闹钟') },
];

/** The transport keys, and the words that press them. */
function Remote({ guide }: { guide: Guide }) {
  return (
    <section className="remote">
      <h2>控制播放，也用说的</h2>
      <ul>
        {['pause', 'resume', 'stop', 'previous', 'next'].filter((x) => guide.controls[x]?.length).map((x) => (
          <li key={x}><Glyph name={x} />{guide.controls[x].map((w, i) => (i ? <span key={w}>{w}</span> : <b key={w}>{w}</b>))}</li>
        ))}
      </ul>
    </section>
  );
}

/** What stays with the speaker's own assistant. */
function Kept({ guide }: { guide: Guide }) {
  const list = shelves.map((x, i) => {
    const known = new Set(shelves.flatMap((y) => y.words));
    return { ...x, words: [...x.words.filter((w) => guide.not_music.includes(w)), ...(i === 0 ? guide.not_music.filter((w) => !known.has(w)) : [])] };
  }).filter((x) => x.words.length);
  const [at, setAt] = useState<Record<string, number>>({});
  return (
    <section className="kept">
      <h2><SourceIcon name="xiaomi" />这些还是小爱来</h2>
      <ul>{list.map((x) => {
        const n = (at[x.key] ?? 0) % x.words.length;
        return (
          <li key={x.key}>
            <button type="button" onClick={() => setAt({ ...at, [x.key]: n + 1 })} aria-label={`${x.name}，换一句`}>
              <Glyph name={x.key} /><b>{x.name}</b>
              <span key={n} className="bubble" aria-live="polite">{x.say(x.words[n])}</span>
              {x.words.length > 1 && <i aria-hidden="true">{x.words.map((w, m) => <u key={w} className={m === n ? 'on' : ''} />)}</i>}
            </button>
          </li>
        );
      })}</ul>
    </section>
  );
}

export function Say({ navigate }: { navigate: (p: Place) => void }) {
  const water = useWater();
  const songs = useSongs();
  const list = useMemo(() => (songs ? ways(songs) : []), [songs]);
  const [i, setI] = useState(0);
  const [hold, setHold] = useState(false);
  const [result, setResult] = useState<{ key: string; route?: Route; error?: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const raft = useRef<HTMLDivElement>(null);
  const way = list[i];
  const [guide, setGuide] = useState<Guide | null>(null);
  useEffect(() => { api.guide().then(setGuide, () => {}); }, []);

  useEffect(() => {
    if (hold || list.length === 0 || matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const t = setInterval(() => { if (!document.hidden) setI((x) => (x + 1) % list.length); }, 5200);
    return () => clearInterval(t);
  }, [hold, list.length]);

  const shown = result && way && result.key === way.key ? result : null;
  const r = shown?.route;
  const p = r && r.decision !== 'native' ? { title: r.title ?? '', artist: r.artist ?? '', cover: r.cover } : r ? undefined : way?.pic;
  const pic = p ? songPicture(p, 1024) : fishPicture;
  const showFish = !p;
  useEffect(() => { if (way) water?.show(pic); }, [water, pic.src, !!way]); // eslint-disable-line react-hooks/exhaustive-deps

  useLayoutEffect(() => {
    if (!water) return;
    const set = (hard: boolean) => {
      const b = raft.current?.getBoundingClientRect();
      if (!b) return;
      const q = showFish ? null : { x: b.left, y: b.top, w: b.width, h: b.height };
      if (hard) water.nudge(q, b.bottom - 2); else water.place(q, b.bottom - 2);
    };
    set(false);
    const f = () => set(true);
    addEventListener('scroll', f, { passive: true });
    addEventListener('resize', f);
    return () => { removeEventListener('scroll', f); removeEventListener('resize', f); };
  }, [water, showFish]);

  const pick = (n: number) => { setHold(true); setI(n); };
  const run = useCallback(async () => {
    if (!way) return;
    setHold(true);
    setBusy(true);
    try {
      const x = await api.tryRoute(said(way));
      setResult({ key: way.key, route: x.route });
    } catch (e) {
      setResult({ key: way.key, error: (e as Error).message });
    } finally {
      setBusy(false);
    }
  }, [way]);

  return (
    <div className="say">
      <section className="scene say-scene" onPointerDown={(e) => { if (water && e.clientY > water.waterline) water.ripple(e.clientX, e.clientY); }}>
        <div className="raft" ref={raft}>{showFish && <Fish water mood={busy ? 'searching' : 'idle'} />}</div>
        {way && (
          <div className="words say-words" key={way.key}>
            <p className="said">小爱同学</p>
            <h1 className="sentence" aria-label={said(way)}>
              {way.parts.map((x, n) => <span key={n} className={x.role ? `part part-${x.role === '口令' ? 'verb' : 'name'}` : 'part'}>{x.text}{x.role && <small>{x.role}</small>}</span>)}
            </h1>
            <p className="by" aria-live="polite">
              {shown?.error ?? (r ? (r.decision === 'native' ? <>交给小爱：{r.reason}</> : <>{stopLabel[r.source ?? 'library']}会放 <b>{r.title}</b>　{r.artist}</>) : way.note)}
            </p>
            <div className="say-act"><button className="btn" disabled={busy} onClick={run}>{busy ? <><span className="spinner spinner-tiny" />正在找</> : '看看会放什么'}</button><span className="muted">不会出声</span></div>
          </div>
        )}
        <div className="say-tabs" role="tablist" aria-label="说法">
          {list.map((w, n) => <button key={w.key} role="tab" aria-selected={n === i} onClick={() => pick(n)} className={!hold && n === i ? 'auto' : ''}>{w.name}</button>)}
        </div>
      </section>

      <section className="log say-more">
        {guide && <><Talk guide={guide} /><Over songs={songs ?? []} /><Openers guide={guide} song={songs?.[0] ?? plain} /></>}
        <Where />
        <Taps navigate={navigate} />
        {guide && <><Remote guide={guide} /><Kept guide={guide} /></>}
        {songs && songs.length > 0 && (
          <div className="say-songs">
            <h2>从这几首开始</h2>
            <ul>{songs.slice(0, 5).map((s) => <li key={s.title + s.artist}><Cover cover={s.cover} title={s.title} artist={s.artist} size={600} /><b>播放{s.artist}的{s.title}</b></li>)}</ul>
            <button className="btn btn-quiet" onClick={() => navigate({ page: 'library' })}>去曲库看看</button>
          </div>
        )}
      </section>
    </div>
  );
}
