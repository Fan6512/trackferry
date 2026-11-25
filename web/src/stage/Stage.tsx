import { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Water, type Picture } from './water';
import { art, coverURL } from './art';
import { bendGlass } from './bend';
import fish from '../assets/brand/fish.webp';

export const fishPicture: Picture = { src: fish, fallback: fish, cutout: true };

export function songPicture(s: { title?: string; artist?: string; cover?: string }, size: 600 | 1024 = 600): Picture {
  const fallback = art(s.title ?? '', s.artist);
  return { src: coverURL(s.cover, size) || fallback, fallback, cutout: false };
}

const Ctx = createContext<Water | null>(null);
export const useWater = () => useContext(Ctx);

export function Stage({ children }: { children: ReactNode }) {
  const cv = useRef<HTMLCanvasElement>(null);
  const [water, setWater] = useState<Water | null>(null);
  useEffect(() => {
    const w = new Water(cv.current!);
    w.show(fishPicture);
    w.place(null, innerHeight * 0.62);
    setWater(w);
    const unbend = bendGlass();
    return () => { w.dispose(); unbend(); };
  }, []);
  const value = useMemo(() => water, [water]);
  return (
    <Ctx.Provider value={value}>
      <canvas ref={cv} className="water" aria-hidden="true" />
      {children}
    </Ctx.Provider>
  );
}

/** Pages without a floating picture: a low horizon and only the colours. */
export function useBackdrop(p?: Picture) {
  const w = useWater();
  useEffect(() => {
    const f = () => w?.place(null, innerHeight * 0.86);
    f();
    addEventListener('resize', f);
    return () => removeEventListener('resize', f);
  }, [w]);
  useEffect(() => { if (p) w?.show(p); }, [w, p?.src]); // eslint-disable-line react-hooks/exhaustive-deps
}
