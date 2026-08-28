export function formatUsdtAmount(value: string): string {
  const normalized = value.trim();
  const match = /^(\d+)(?:\.(\d+))?$/.exec(normalized);
  if (!match) return value;

  const [, integer, fraction = ''] = match;
  if (fraction.length > 4 && /[1-9]/.test(fraction.slice(4))) return normalized;

  return `${integer}.${fraction.slice(0, 4).padEnd(4, '0')}`;
}
