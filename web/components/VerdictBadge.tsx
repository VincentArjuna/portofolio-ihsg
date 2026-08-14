// Verdict badge: Buy=green, Hold=yellow, Sell=red (docs/DESIGN.md).
// 15% tinted bg, 30% border, 4px radius. Full class strings so Tailwind's
// scanner keeps them (no dynamic interpolation).

const STYLES: Record<string, string> = {
  BUY: "border-success/30 bg-success/15 text-success",
  HOLD: "border-warning/30 bg-warning/15 text-warning",
  SELL: "border-danger/30 bg-danger/15 text-danger",
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
      <span className="tnum rounded bg-surface-2 px-2 py-0.5 text-xs text-muted">
        —
      </span>
    );
  }
  const cls = STYLES[verdict] ?? "border-line bg-surface-2 text-muted";
  const sizing = size === "md" ? "px-2.5 py-1 text-sm" : "px-2 py-0.5 text-xs";
  return (
    <span className={`tnum inline-flex items-center rounded border font-medium ${cls} ${sizing}`}>
      {LABELS[verdict] ?? verdict}
    </span>
  );
}
