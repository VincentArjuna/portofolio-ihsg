// Indonesian-locale formatters (DESIGN: tabular numbers, IDR, id-ID).

const idr0 = new Intl.NumberFormat("id-ID", {
  style: "currency",
  currency: "IDR",
  maximumFractionDigits: 0,
});
const num2 = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });
const num0 = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 0 });
const dateFmt = new Intl.DateTimeFormat("id-ID", {
  day: "2-digit",
  month: "short",
  year: "numeric",
});

export const formatIDR = (n: number) => idr0.format(n);
export const formatNum = (n: number) => num0.format(n);
export const formatPct = (n: number) => `${num2.format(n)}%`;
export const formatDate = (s: string) => {
  // Treat the YYYY-MM-DD string as a local date (avoid TZ shifting the day).
  const d = new Date(`${s}T00:00:00`);
  return Number.isNaN(d.getTime()) ? s : dateFmt.format(d);
};

/** Render a nullable market-dependent value as an em-dash placeholder. */
export const orDash = (v: number | null | undefined) =>
  v === null || v === undefined ? "—" : v;
