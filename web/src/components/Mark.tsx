import fish from '../assets/brand/fish-mark.webp';
import word from '../assets/brand/wordmark.svg';

export function Brand() {
  return <><img className="brand-fish" src={fish} alt="" width={44} height={44} /><img className="brand-word" src={word} alt="" width={118} height={26} /></>;
}
