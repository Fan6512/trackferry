import './signature.css';

/** The author's mark: one quiet line that lights up when pointed at. */
export function Signature() {
  return (
    <a className="sig" href="https://github.com/madrays" target="_blank" rel="noreferrer" aria-label="作者 madrays，打开 GitHub 主页">
      <span className="sig-by">Crafted by</span>
      <span className="sig-name">madrays</span>
      <svg viewBox="0 0 10 10" width="9" height="9" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M2 8 8 2M3 2h5v5" /></svg>
    </a>
  );
}
