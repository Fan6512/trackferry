import type { Route } from '../api';
import { layerSays, ms, song, stopLabel } from '../route';
import { length } from '../copy';
import { Cover } from './Cover';
import { SourceIcon } from './SourceIcon';

/** What the decision rests on, step by step. */
export function Basis({ route }: { route: Route }) {
  if (!route.basis?.length) return null;
  return <ol className="basis">{route.basis.map((f, i) => <li key={i}><span>{f.name}</span><b>{f.text}</b></li>)}</ol>;
}

/** What each source answered. */
export function RouteDetail({ route }: { route: Route }) {
  const asked = route.layers.filter((l) => l.name !== 'xiaomi');
  return (
    <div className="layers">
      {asked.some((l) => l.opinion) && <p className="muted opinion-note">曲库里有同名歌，线上只问原唱是谁，没有取歌。</p>}
      {asked.map((l) => (
        <div key={l.name} className={`layer layer-${l.opinion ? 'opinion' : l.state}`}>
          <div className="layer-head">
            <b><SourceIcon name={l.name} />{stopLabel[l.name]}</b>
            <span>{layerSays(l)}</span>
            {l.state !== 'skipped' && l.state !== 'disabled' && <span className="num muted">{ms(l.ms)}</span>}
          </div>
          {l.candidates?.length > 0 && (
            <ul className="cands">
              {l.candidates.map((c, i) => (
                <li key={i}>
                  <Cover cover={c.cover} title={c.title} artist={c.artist} />
                  <span className="cand-main"><span>{song(c)}</span>{c.detail && <small>{c.detail}</small>}</span>
                  {c.duration_ms ? <span className="num muted">{length(c.duration_ms)}</span> : null}
                </li>
              ))}
            </ul>
          )}
        </div>
      ))}
      <div className="layer">
        <div className="layer-head"><b><SourceIcon name="xiaomi" />小爱</b><span>{route.decision === 'native' ? (route.reason || '由小爱处理') : '没有用到'}</span></div>
      </div>
    </div>
  );
}
