import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react';

export interface Choice<T> { value: T; label: string }

/** One choice out of a few, in the app's own look. The button keeps the focus; arrows move the mark. */
export function Pick<T extends string | number>({ value, choices, label, onPick }: { value: T; choices: Choice<T>[]; label: string; onPick: (v: T) => void }) {
  const id = useId();
  const box = useRef<HTMLSpanElement>(null);
  const [at, setAt] = useState<number | null>(null); // the marked choice while open
  const [down, setDown] = useState(false);
  const open = at !== null;
  const now = Math.max(0, choices.findIndex((c) => c.value === value));
  useEffect(() => {
    if (!open) return;
    const off = (e: Event) => { if (!box.current?.contains(e.target as Node)) setAt(null); };
    addEventListener('pointerdown', off);
    return () => removeEventListener('pointerdown', off);
  }, [open]);
  const show = () => {
    setDown((box.current?.getBoundingClientRect().top ?? 0) < innerHeight / 2);
    setAt(now);
  };
  const take = (i: number) => {
    setAt(null);
    if (choices[i].value !== value) onPick(choices[i].value);
  };
  const key = (e: KeyboardEvent) => {
    const last = choices.length - 1;
    const to: Record<string, number> = { ArrowDown: Math.min(last, (at ?? 0) + 1), ArrowUp: Math.max(0, (at ?? 0) - 1), Home: 0, End: last };
    if (!open) {
      if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
      show();
    } else if (e.key in to) setAt(to[e.key]);
    else if (e.key === 'Enter' || e.key === ' ') take(at);
    else if (e.key === 'Escape') setAt(null);
    else return;
    e.preventDefault();
  };
  return (
    <span className="pick" ref={box}>
      <button type="button" className="pick-btn" role="combobox" aria-label={label} aria-haspopup="listbox" aria-expanded={open} aria-controls={open ? id : undefined}
        aria-activedescendant={open ? `${id}-${at}` : undefined} onClick={() => (open ? setAt(null) : show())} onKeyDown={key} onBlur={() => setAt(null)}>
        <span>{choices[now]?.label}</span>
        <svg viewBox="0 0 12 8" width="12" height="8" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M1.5 1.5 6 6l4.5-4.5" /></svg>
      </button>
      {open && (
        <span className={`menu pick-menu ${down ? 'menu-down' : ''}`} role="listbox" id={id} aria-label={label} onPointerDown={(e) => e.preventDefault()} onMouseDown={(e) => e.preventDefault()}>
          {choices.map((c, i) => (
            <span key={c.value} id={`${id}-${i}`} role="option" aria-selected={c.value === value} className={i === at ? 'at' : ''} onPointerMove={() => { if (i !== at) setAt(i); }} onClick={() => take(i)}>
              {c.label}
              {c.value === value && <svg viewBox="0 0 14 10" width="14" height="10" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M1.5 5 5 8.5l7.5-7" /></svg>}
            </span>
          ))}
        </span>
      )}
    </span>
  );
}
