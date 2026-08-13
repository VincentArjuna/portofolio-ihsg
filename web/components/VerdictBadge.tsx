// Verdict badge: Buy=green, Hold=yellow, Sell=red (docs/DESIGN.md).
// Full class strings so Tailwind's scanner keeps them (no dynamic interpolation).

const STYLES: Record<string, string> = {
  BUY: "border-success/40 bg-success/10 text-success",
  HOLD: "border-warning/40 bg-warning/10 text-warning",
  SELL: "border-danger/40 bg-danger/10 text-danger",
};

const LABELS: Record<string, string> = {
  BUY: "Beli",
  HOLD: "Tahan",
  SELL: "Jual",
};

export function verdictLabel(v: string | null | undefined): string {
  if (!v) return "—";
  return LABELS[v] ?? v;
}

export default function VerdictBadge({
  verdict,
  size = "sm",
}: {
  verdict: string | null | undefined;
  size?: "sm" | "md";
}) {
  if (!verdict) {
    return (
      <span className="tnum rounded-md bg-base/60 px-2 py-0.5 text-xs text-muted">
        —
      </span>
    );
  }
  const cls = STYLES[verdict] ?? "border-edge bg-base/60 text-muted";
  const sizing = size === "md" ? "px-2.5 py-1 text-sm" : "px-2 py-0.5 text-xs";
  return (
    <span className={`tnum inline-flex items-center rounded-md border font-semibold ${cls} ${sizing}`}>
      {LABELS[verdict] ?? verdict}
    </span>
  );
}
