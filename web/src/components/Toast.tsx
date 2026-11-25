import { useEffect, useState } from 'react';
import { createPortal } from 'react-dom';

export type Said = { tone: 'busy' | 'ok' | 'bad'; text: string };

/** Says how something went; a tap takes it away. */
export function useToast(): [React.ReactNode, (s: Said | null) => void] {
  const [state, setState] = useState<Said | null>(null);
  useEffect(() => {
    if (!state || state.tone === 'busy') return;
    const t = setTimeout(() => setState(null), state.tone === 'bad' ? 12000 : 3500);
    return () => clearTimeout(t);
  }, [state]);
  return [state && createPortal(<button type="button" className={`play-state ${state.tone}`} title="点一下关闭" onClick={() => { if (state.tone !== 'busy') setState(null); }}><span role={state.tone === 'bad' ? 'alert' : 'status'}>{state.text}</span></button>, document.body), setState];
}
