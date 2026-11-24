import type { TakeoverRun } from '../api';
import { landedAt, outcome, stepLabel } from '../route';
import { clock, secs } from '../copy';

export function RunSteps({ run }: { run: TakeoverRun }) {
  const landed = landedAt(run);
  const lead = run.upstream_ms ? run.started_ms - run.upstream_ms : 0;
  return (
    <div className="runsteps">
      {landed !== undefined && (
        <p>开始接管 <b className="num">{secs(landed)} 秒</b>后放上{lead > 0 && <>，从小爱听清算起 <b className="num">{secs(lead + landed)} 秒</b></>}</p>
      )}
      {run.detail && <p className="muted">{run.detail}</p>}
      <ol>
        {(run.steps ?? []).map((s, i) => (
          <li key={i} className={s.ok ? '' : 'bad'}>
            <span className="num">{secs(s.at_ms)}</span>
            <span>{stepLabel(s.step)}</span>
            <span className="muted">{s.detail}</span>
          </li>
        ))}
      </ol>
    </div>
  );
}

export function RunRow({ run }: { run: TakeoverRun }) {
  const o = outcome(run);
  return (
    <li>
      <details className="run">
        <summary>
          <span className="num">{clock(run.started_ms)}</span>
          <span>{run.device_name || run.device_id}</span>
          <span className="muted run-src">{run.source}</span>
          <span className={`tag tag-${o.tone}`}>{run.outcome === 'running' && <span className="spinner spinner-tiny" />}{o.text}</span>
        </summary>
        <RunSteps run={run} />
      </details>
    </li>
  );
}
