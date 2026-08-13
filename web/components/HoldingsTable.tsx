import type { Position } from "@/lib/api";
import { formatIDR, formatShares, formatPct, orDash } from "@/lib/format";
import VerdictBadge from "@/components/VerdictBadge";

interface Props {
  positions: Position[];
  loading: boolean;
  onEdit: (p: Position) => void;
  onDelete: (p: Position) => void;
  onDetail: (p: Position) => void;
  onAnalyze: (p: Position) => void;
}

const COLS = [
  "Ticker",
  "Saham",
  "Harga Beli",
  "Alokasi",
  "Harga Kini",
  "Gain/Loss",
  "Verdict ST",
  "Verdict LT",
  "Aksi",
];

function Placeholder({ label = "menunggu data" }: { label?: string }) {
  return (
    <span
      className="tnum rounded-md bg-base/60 px-2 py-0.5 text-xs text-muted"
      title={label}
    >
      —
    </span>
  );
}

// Verdict badge + an explicit "beda pendapat" marker when rule disagrees with
// AI (docs/DESIGN.md signature element). The marker lights up once a Hermes AI
// run (T4) has stored a verdict that differs from the rule verdict.
function VerdictCell({ verdict, disagreement }: { verdict: string | null; disagreement?: boolean }) {
  if (!verdict) return <Placeholder label="verdict hadir setelah data pasar dimuat" />;
  return (
    <div className="flex items-center gap-1.5">
      <VerdictBadge verdict={verdict} />
      {disagreement && (
        <span
          className="rounded bg-warning/15 px-1 py-0.5 text-[10px] font-semibold uppercase text-warning"
          title="Aturan dan AI berbeda pendapat"
        >
          ⚠
        </span>
      )}
    </div>
  );
}

function SkeletonRow() {
  return (
    <tr className="border-t border-edge">
      {Array.from({ length: COLS.length }).map((_, i) => (
        <td key={i} className="px-4 py-3">
          <div className="skeleton h-4 w-full max-w-[6rem] rounded" />
        </td>
      ))}
    </tr>
  );
}

export default function HoldingsTable({ positions, loading, onEdit, onDelete, onDetail, onAnalyze }: Props) {
  return (
    <div className="overflow-x-auto rounded-lg border border-edge">
      <table className="w-full min-w-[920px] border-collapse text-sm">
        <thead className="bg-surface">
          <tr>
            {COLS.map((c) => (
              <th
                key={c}
                className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-muted"
              >
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {loading ? (
            Array.from({ length: 3 }).map((_, i) => <SkeletonRow key={i} />)
          ) : (
            positions.map((p) => {
              const basis = p.shares * p.avg_buy_price;
              return (
                <tr
                  key={p.id}
                  className="border-t border-edge transition-colors hover:bg-surface/60"
                >
                  <td className="px-4 py-3 font-semibold tracking-wide text-ink">
                    {p.ticker}
                  </td>
                  <td className="tnum px-4 py-3 text-ink">{formatShares(p.shares)}</td>
                  <td className="tnum px-4 py-3 text-ink">{formatIDR(p.avg_buy_price)}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <span className="tnum w-12 text-muted">
                        {formatPct(p.weight_pct)}
                      </span>
                      <div className="h-1.5 w-16 overflow-hidden rounded-full bg-base">
                        <div
                          className="h-full rounded-full bg-accent"
                          style={{ width: `${Math.min(p.weight_pct, 100)}%` }}
                        />
                      </div>
                    </div>
                  </td>
                  <td className="tnum px-4 py-3 text-muted">
                    {p.current_price === null ? (
                      <Placeholder />
                    ) : (
                      formatIDR(p.current_price)
                    )}
                  </td>
                  <td className="tnum px-4 py-3 text-muted">
                    {p.profit_loss_idr === null ? (
                      <Placeholder />
                    ) : (
                      <span
                        className={
                          p.profit_loss_idr > 0
                            ? "text-success"
                            : p.profit_loss_idr < 0
                              ? "text-danger"
                              : "text-ink"
                        }
                      >
                        {formatIDR(p.profit_loss_idr)} ({formatPct(orDash(p.profit_loss_pct) as number)})
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <VerdictCell verdict={p.verdicts?.short_term.rule ?? null} disagreement={p.verdicts?.short_term.disagreement} />
                  </td>
                  <td className="px-4 py-3">
                    <VerdictCell verdict={p.verdicts?.long_term.rule ?? null} disagreement={p.verdicts?.long_term.disagreement} />
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex gap-1.5">
                      <button
                        type="button"
                        onClick={() => onAnalyze(p)}
                        className="rounded-md border border-accent/50 px-2.5 py-1 text-xs font-medium text-accent transition-colors hover:bg-accent/10"
                        title="Jalankan analisis Hermes AI"
                      >
                        Analisis AI
                      </button>
                      <button
                        type="button"
                        onClick={() => onDetail(p)}
                        className="rounded-md border border-edge px-2.5 py-1 text-xs font-medium text-muted transition-colors hover:border-accent hover:text-accent"
                      >
                        Detail
                      </button>
                      <button
                        type="button"
                        onClick={() => onEdit(p)}
                        className="rounded-md border border-edge px-2.5 py-1 text-xs font-medium text-muted transition-colors hover:border-accent hover:text-accent"
                      >
                        Edit
                      </button>
                      <button
                        type="button"
                        onClick={() => onDelete(p)}
                        className="rounded-md border border-edge px-2.5 py-1 text-xs font-medium text-muted transition-colors hover:border-danger hover:text-danger"
                      >
                        Hapus
                      </button>
                    </div>
                  </td>
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}
