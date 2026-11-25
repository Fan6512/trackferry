import { useEffect, useRef, useState } from 'react';

/** The end of a long list: asks for more before it is reached. */
export function More({ busy = false, onMore }: { busy?: boolean; onMore: () => void }) {
  const end = useRef<HTMLDivElement>(null);
  const ask = useRef(onMore);
  ask.current = onMore;
  const [near, setNear] = useState(false);
  useEffect(() => {
    const io = new IntersectionObserver((es) => setNear(es[es.length - 1].isIntersecting), { rootMargin: '0px 0px 800px' });
    io.observe(end.current!);
    return () => io.disconnect();
  }, []);
  // Still in sight after a page: ask again, a little later.
  useEffect(() => {
    if (!near || busy) return;
    ask.current();
    const t = setInterval(() => ask.current(), 250);
    return () => clearInterval(t);
  }, [near, busy]);
  return <div ref={end} className="more-end"><span className="spinner" /></div>;
}

/** The first rows of a list at hand, growing as it is scrolled. */
export function useGrow(step = 60) {
  const [shown, setShown] = useState(step);
  return { shown, grow: () => setShown((n) => n + step) };
}
