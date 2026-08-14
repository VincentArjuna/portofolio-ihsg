import type { Summary } from "@/lib/api";
import { formatDateTime, formatIDR, formatPct, isStale } from "@/lib/format";

interface Props {
  summary: Summary;
  positionCount: number;
}

type Tone = "ink" | "accent" | "success" | "warning" | "danger" | "muted";

const toneText: Record<Tone, string> = {
  ink: "text-ink",
  accent: "text-accent",
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
  muted: "text-muted",
};

function Card({
  label,
  value,
  sub,
  tone = "ink",
}: {
  label: string;
  value: string;
  sub?: string;
  tone?: Tone;
}) {
  return (
    <div className="rounded-lg border border-edge bg-surface-1 p-4 transition-colors hover:bg-surface-2">
      <p className="text-xs font-medium uppercase tracking-wide text-muted">
        {label}
      </p>
      <p className={`tnum mt-2 text-[28px] font-medium leading-tight tracking-tight ${toneText[tone]}`}>{value}</p>
      {sub && <p className="tnum mt-1 text-xs text-muted">{sub}</p>}
    </div>
  );
}

export default function KpiCards({ summary, positionCount }: Props) {
  const pl = summary.total_profit_loss_idr;
  const plTone: Tone =
    pl === null ? "muted" : pl > 0 ? "success" : pl < 0 ? "danger" : "ink";
  const stale = isStale(summary.last_market_update);

  return (
    <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
      <Card
        label="Total Investasi"
        value={formatIDR(summary.total_investment_idr)}
        sub={`${positionCount} posisi saham`}
      />
      <Card
        label="Nilai Portofolio"
        value={summary.current_value_idr === null ? "—" : formatIDR(summary.current_value_idr)}
        sub={summary.current_value_idr === null ? "belum ada data harga" : "nilai pasar kini"}
        tone={summary.current_value_idr === null ? "muted" : "accent"}
      />
      <Card
        label="Total Gain/Loss"
        value={pl === null ? "—" : formatIDR(pl)}
        sub={
          summary.total_profit_loss_pct === null
            ? "belum ada data harga"
            : formatPct(summary.total_profit_loss_pct)
        }
        tone={plTone}
      />
      <Card
        label="Status Data Pasar"
        value={
          summary.last_market_update === null
            ? "Belum ada"
            : stale
              ? "Data basi"
              : "Segar"
        }
        sub={
          summary.last_market_update === null
            ? "tekan Perbarui Data"
            : formatDateTime(summary.last_market_update)
        }
        tone={
          summary.last_market_update === null
            ? "muted"
            : stale
              ? "warning"
              : "success"
        }
      />
    </div>
  );
}
