/** Parse a CSS hex color string (e.g. '#2194ce') to an integer. */
export function hexToInt(s: string): number {
  const n = parseInt(s.replace('#', ''), 16);
  return isNaN(n) ? 0 : n;
}
