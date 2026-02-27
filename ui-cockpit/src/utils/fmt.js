/**
 * Format Indian currency: ₹X.XX Cr, ₹X.XL, or ₹X,XXX
 */
export function fmt(n) {
  const abs = Math.abs(n);
  if (abs >= 10000000) return `₹${(n / 10000000).toFixed(2)} Cr`;
  if (abs >= 100000) return `₹${(n / 100000).toFixed(1)}L`;
  return `₹${Math.round(abs).toLocaleString("en-IN")}`;
}

export function fmtCr(n) {
  return `₹${(n / 10000000).toFixed(2)} Cr`;
}
