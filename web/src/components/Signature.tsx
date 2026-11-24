import { type CSSProperties } from 'react';
import fish from '../assets/brand/fish-mark.webp';
import './signature.css';

const NAME = 'madrays';
const letters = [...NAME].map((c, i) => <i key={i} style={{ '--i': i } as CSSProperties}>{c}</i>);

/** The ferryman's name, afloat: it bobs on the water and the water shows it back. */
export function Signature() {
  return (
    <a className="sig" href="https://github.com/madrays" target="_blank" rel="noreferrer" aria-label="作者 madrays，打开 GitHub 主页">
      <svg width="0" height="0" aria-hidden="true"><filter id="sig-ripple" x="-5%" y="-10%" width="110%" height="120%"><feTurbulence type="fractalNoise" baseFrequency=".012 .16" numOctaves="2" seed="7" /><feDisplacementMap in="SourceGraphic" scale="9" /></filter></svg>
      <span className="sig-role" aria-hidden="true">摆渡人</span>
      <span className="sig-sea" aria-hidden="true">
        <img className="sig-fish" src={fish} alt="" width={38} height={38} />
        <span className="sig-name">{letters}</span>
        <span className="sig-line"><svg viewBox="0 0 480 10" preserveAspectRatio="none"><path d={`M0 5${' q15 -5 30 0 t30 0'.repeat(8)}`} /></svg></span>
        <span className="sig-glass"><span className="sig-name">{letters}</span></span>
      </span>
    </a>
  );
}
