import { useLayoutEffect, useRef, type ReactNode } from 'react';
import { Jelly } from './jelly';

/** Two or three to choose from, with the same drop as the bar of a phone. */
export function Segments<T extends string>({ label, value, items, onChange }: { label: string; value: T; items: { value: T; label: ReactNode }[]; onChange: (v: T) => void }) {
  const box = useRef<HTMLDivElement>(null), lens = useRef<HTMLSpanElement>(null), jelly = useRef<Jelly>(null);
  useLayoutEffect(() => {
    const b = box.current!, j = jelly.current = new Jelly(b, lens.current!, () => [...b.querySelectorAll('button')]);
    const ro = new ResizeObserver(() => { const a = b.querySelector<HTMLElement>('[aria-selected=true]'); if (a && b.offsetWidth !== was) { was = b.offsetWidth; j.put(a.offsetLeft, a.offsetWidth); } });
    let was = b.offsetWidth;
    ro.observe(b);
    return () => { ro.disconnect(); j.stop(); };
  }, []);
  useLayoutEffect(() => { const a = box.current!.querySelector<HTMLElement>('[aria-selected=true]'); if (a) jelly.current!.go(a.offsetLeft, a.offsetWidth); }, [value]);
  const free = () => jelly.current!.hold(false);
  return (
    <div ref={box} className="sides glass" role="tablist" aria-label={label} onPointerDown={() => jelly.current!.hold(true)} onPointerUp={free} onPointerCancel={free} onPointerLeave={free}>
      <span ref={lens} className="lens" aria-hidden="true"><i /><i /></span>
      {items.map((i) => <button key={i.value} type="button" role="tab" aria-selected={value === i.value} onClick={() => onChange(i.value)}>{i.label}</button>)}
    </div>
  );
}
