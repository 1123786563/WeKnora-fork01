/** Presentation only. Never convert ledger integers to IEEE-754 numbers. */
function digits(value: string): string {
  if (typeof value !== 'string' || !/^\d+$/.test(value)) throw new TypeError('Expected an unsigned integer string');
  return value.replace(/^0+(?=\d)/, '');
}
export function formatCredits(value: string): string { return digits(value).replace(/\B(?=(\d{3})+(?!\d))/g, ','); }
export function formatMoney(fen: string): string {
  const value = digits(fen).padStart(3, '0');
  return `${formatCredits(value.slice(0, -2))}.${value.slice(-2)}`;
}
export function formatSignedCredits(value: string): string {
  return value.startsWith('-') ? `−${formatCredits(value.slice(1))}` : formatCredits(value);
}
export function formatTime(value?: string | null): string {
  if (!value) return '—';
  const d = new Date(value); if (Number.isNaN(d.getTime())) return '—';
  const pad = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '—';
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}
