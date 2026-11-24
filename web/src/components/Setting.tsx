import type { ReactNode } from 'react';

export type Light = 'ok' | 'warn' | 'bad' | 'off';

/** One subject of the settings page: what it is, how it stands, what can be done. */
export function Setting({ id, icon, title, says, light, state, actions, children }: {
  id: string; icon: ReactNode; title: string; says?: string; light?: Light; state?: ReactNode; actions?: ReactNode; children?: ReactNode;
}) {
  return (
    <section className="st" id={`set-${id}`}>
      <header className="st-head">
        <span className="st-icon">{icon}</span>
        <div><h2>{title}{state && <em className={`st-state ${light ?? 'off'}`}>{state}</em>}</h2>{says && <p>{says}</p>}</div>
        {actions && <div className="st-actions">{actions}</div>}
      </header>
      {children && <div className="st-body">{children}</div>}
    </section>
  );
}

const draw = (d: ReactNode) => <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{d}</svg>;
export const LockIcon = () => draw(<><rect x="5" y="10.5" width="14" height="9.5" rx="2" /><path d="M8 10.5V8a4 4 0 0 1 8 0v2.5M12 14.5v2" /></>);
export const WavesIcon = () => draw(<path d="M3 8.5c3-2.4 6-2.4 9 0s6 2.4 9 0M3 15.5c3-2.4 6-2.4 9 0s6 2.4 9 0" />);
