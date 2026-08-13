"use client";

import { useCallback, useEffect, useState } from "react";
import {
  analyzeStockAI,
  fetchStockDetail,
  refreshMarketData,
  type StockDetail,
  type StockHorizon,
} from "@/lib/api";
import { formatDateTime, formatIDR, formatPct } from "@/lib/format";
import VerdictBadge, { verdictLabel } from "@/components/VerdictBadge";

interface Props {
  ticker: string | null; // when set, modal opens for this ticker
  autoAnalyze?: boolean; // T4: trigger "Analisis AI" immediately on open
  onClose: () => void;
}

// Ordered factors + max weight per horizon, for breakdown bars + rationale.
const SHORT_FACTORS: { key: string; label: string; max: number }[] = [
  { key: "trend_teknis", label: "Tren Teknis", max: 30 },
  { key: "momentum", label: "Momentum", max: 25 },
  { key: "volume", label: "Volume", max: 15 },
  { key: "valuasi", label: "Valuasi", max: 15 },
  { key: "earnings_momentum", label: "Momentum Laba", max: 15 },
];
const LONG_FACTORS: { key: string; label: string; max: number }[] = [
  { key: "profitabilitas", label: "Profitabilitas", max: 30 },
  { key: "solvabilitas", label: "Solvabilitas", max: 20 },
  { key: "valuasi", label: "Valuasi", max: 20 },
  { key: "pertumbuhan", label: "Pertumbuhan", max: 15 },
  { key: "trend_teknis", label: "Tren Teknis", max: 15 },
];

const RISK_LABELS: Record<string, string> = {
  high_debt: "Utang tinggi (DER > 2.0)",
  low_profitability: "Profitabilitas rendah (ROE < 8%)",
  overvalued: "Overvalued (PER > 25 / PBV > 3)",
  downtrend: "Tren menurun (harga < MA50 < MA200)",
};

const f2 = (n: number) => n.toFixed(2);

// rationale builds a one-line Indonesian verdict rationale from the breakdown:
// the strongest and weakest contributing factors.
function rationale(h: StockHorizon, factors: { key: string; label: string; max: number }[]): string {
  if (!h.rule) return "Belum ada skor rule untuk horizon ini.";
  const verb =
    h.rule.verdict === "BUY"
      ? "mendukung pembelian"
      : h.rule.verdict === "SELL"
        ? "menyarankan penjualan"
        : "netral (tahan)";
  const entries = factors
    .map((f) => ({ label: f.label, v: h.rule!.breakdown[f.key] ?? 0, max: f.max ?? 0 }))
    .filter((e) => e.max > 0);
  if (entries.length === 0) return `Skor ${h.rule.score}/100 → ${h.rule.verdict}.`;
  const top = entries.reduce((a, b) => (b.v / b.max > a.v / a.max ? b : a));
  const low = entries.reduce((a, b) => (b.v / b.max < a.v / a.max ? b : a));
  const topShare = Math.round((top.v / top.max) * 100);
  const lowShare = Math.round((low.v / low.max) * 100);
  return `Skor ${h.rule.score}/100 ${verb}. Terkuat: ${top.label} (${topShare}%). Terlemah: ${low.label} (${lowShare}%).`;
}

export default function StockDetailModal({ ticker, autoAnalyze, onClose }: Props) {
  const [data, setData] = useState<StockDetail | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [analyzing, setAnalyzing] = useState(false);
  const [aiMessage, setAiMessage] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const load = useCallback(async (t: string) => {
    setLoading(true);
    setError(null);
    setData(null);
    setAiMessage(null);
    try {
      setData(await fetchStockDetail(t));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memuat detail saham");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!ticker) return;
    load(ticker);
  }, [ticker, load]);

  // refreshAndReload fetches market data for all held tickers, then reloads
  // this ticker's detail so needs_refresh clears once data is present.
  const refreshAndReload = useCallback(async () => {
    if (!ticker || refreshing) return;
    setRefreshing(true);
    setAiMessage(null);
    try {
      await refreshMarketData();
      await load(ticker);
    } catch (e) {
      setAiMessage(e instanceof Error ? e.message : "Pembaruan data pasar gagal.");
    } finally {
      setRefreshing(false);
    }
  }, [ticker, refreshing, load]);

  // runAnalysis triggers the Hermes AI bridge and merges the returned detail.
  const runAnalysis = useCallback(async () => {
    if (!ticker || analyzing) return;
    setAnalyzing(true);
    setAiMessage(null);
    try {
      const res = await analyzeStockAI(ticker);
      if (res.detail) {
        setData(res.detail);
      }
      if (res.status === "unavailable" || res.status === "error") {
        setAiMessage(res.message ?? "Analisis AI tidak dapat dijalankan.");
      }
    } catch (e) {
      setAiMessage(e instanceof Error ? e.message : "Analisis AI gagal.");
    } finally {
      setAnalyzing(false);
    }
  }, [ticker, analyzing]);

  // Auto-trigger when opened via the "Analisis AI" row button and no AI yet.
  useEffect(() => {
    if (!ticker || !autoAnalyze) return;
    if (data && !data.short_term.ai && !data.long_term.ai) {
      runAnalysis();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ticker, autoAnalyze, data]);

  useEffect(() => {
    if (!ticker) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [ticker, onClose]);

  if (!ticker) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="detail-title"
        onClick={(e) => e.stopPropagation()}
        className="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-lg border border-edge bg-surface p-6 shadow-xl"
      >
        <div className="flex items-start justify-between gap-4">
          <div>
            <h2 id="detail-title" className="text-xl font-bold tracking-tight text-ink">
              {ticker}
            </h2>
            <p className="mt-0.5 text-sm text-muted">
              {data?.company_name ?? "Detail saham & analisis rule"}
            </p>
          </div>
          <div className="flex shrink-0 items-center gap-2">
            <button
              type="button"
              onClick={runAnalysis}
              disabled={analyzing || loading}
              className="rounded-md bg-accent px-3 py-1.5 text-sm font-semibold text-base transition-colors hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {analyzing ? "Menganalisis..." : "Analisis AI"}
            </button>
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border border-edge px-3 py-1.5 text-sm font-medium text-muted transition-colors hover:text-ink"
            >
              Tutup
            </button>
          </div>
        </div>

        {/* AI generating / fallback state (DESIGN: "Hermes sedang menganalisis...") */}
        {analyzing && (
          <div className="mt-4 flex items-center gap-2 rounded-md border border-accent/40 bg-accent/10 px-3 py-2 text-sm text-accent">
            <span className="inline-block h-3 w-3 animate-spin rounded-full border-2 border-accent border-t-transparent" />
            Hermes sedang menganalisis data...
          </div>
        )}
        {aiMessage && !analyzing && (
          <p className="mt-4 rounded-md border border-warning/40 bg-warning/10 px-3 py-2 text-sm text-warning">
            {aiMessage}
          </p>
        )}

        {loading && (
          <div className="mt-6 space-y-2">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="skeleton h-5 w-full rounded" />
            ))}
          </div>
        )}

        {error && (
          <p className="mt-6 rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">
            {error}
          </p>
        )}

        {data && data.needs_refresh && (
          <div className="mt-6 rounded-md border border-warning/40 bg-warning/10 px-4 py-3 text-sm text-warning">
            <p>Belum ada data pasar. Klik Perbarui Data untuk mengambil data terbaru.</p>
            <button
              type="button"
              onClick={refreshAndReload}
              disabled={refreshing || loading}
              className="mt-2 rounded-md bg-accent px-3 py-1.5 text-sm font-semibold text-base transition-colors hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {refreshing ? "Memperbarui..." : "Perbarui Data"}
            </button>
          </div>
        )}

        {data && !data.needs_refresh && (
          <div className="mt-6 space-y-6">
            {/* Market data + technicals */}
            <section>
              <h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-muted">
                Data Pasar & Fundamental
              </h3>
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4">
                <Metric label="Harga Kini" value={formatIDR(data.market_data.last_price)} />
                <Metric label="Harga Tutup" value={formatIDR(data.market_data.prev_close)} />
                <Metric label="PER" value={f2(data.market_data.pe_ratio)} />
                <Metric label="PBV" value={f2(data.market_data.pbv_ratio)} />
                <Metric label="ROE" value={formatPct(data.market_data.roe)} />
                <Metric label="DER" value={f2(data.market_data.der)} />
                <Metric label="Pert. Revenue" value={formatPct(data.market_data.rev_growth)} />
                <Metric label="Net Margin" value={formatPct(data.market_data.net_margin)} />
                <Metric label="MA20" value={formatIDR(data.market_data.ma20)} />
                <Metric label="MA50" value={formatIDR(data.market_data.ma50)} />
                <Metric label="MA200" value={formatIDR(data.market_data.ma200)} />
                <Metric label="Diperbarui" value={formatDateTime(data.market_data.updated_at)} />
              </div>
            </section>

            <HorizonCard title="Jangka Pendek" horizon={data.short_term} factors={SHORT_FACTORS} analyzing={analyzing} />
            <HorizonCard title="Jangka Panjang" horizon={data.long_term} factors={LONG_FACTORS} analyzing={analyzing} />

            <p className="text-xs text-muted">
              Verdict rule deterministik (Balanced-Growth). Verdict AI dari
              Hermes CLI lokal dan disimpan 24 jam. Harga tertunda. Bukan saran
              investasi.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-edge bg-base/40 px-3 py-2">
      <p className="text-[10px] font-medium uppercase tracking-wide text-muted">{label}</p>
      <p className="tnum mt-0.5 text-sm font-semibold text-ink">{value}</p>
    </div>
  );
}

function HorizonCard({
  title,
  horizon,
  factors,
  analyzing,
}: {
  title: string;
  horizon: StockHorizon;
  factors: { key: string; label: string; max: number }[];
  analyzing: boolean;
}) {
  const ai = horizon.ai;
  return (
    <section className="rounded-lg border border-edge bg-base/30 p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <h3 className="text-sm font-semibold text-ink">{title}</h3>
          <span className="text-xs text-muted">{horizon.horizon}</span>
        </div>
        {horizon.rule && (
          <div className="flex items-center gap-2">
            <span className="tnum text-sm font-semibold text-muted">
              {horizon.rule.score}/100
            </span>
            <VerdictBadge verdict={horizon.rule.verdict} size="md" />
          </div>
        )}
      </div>

      {/* Disagreement banner (DESIGN signature: "Beda Pendapat: ...") */}
      {horizon.disagreement && horizon.rule && ai?.verdict && (
        <div className="mt-2 rounded-md border border-warning/50 bg-warning/10 px-3 py-1.5 text-xs font-medium text-warning">
          ⚠ Beda Pendapat: Aturan {verdictLabel(horizon.rule.verdict)} vs AI {verdictLabel(ai.verdict)}
        </div>
      )}

      <div className="mt-3 grid gap-4 md:grid-cols-2">
        {/* Rule side */}
        <div>
          {horizon.rule ? (
            <div className="space-y-1.5">
              {factors.map((f) => {
                const v = horizon.rule!.breakdown[f.key] ?? 0;
                const pct = Math.max(0, Math.min(100, (v / f.max) * 100));
                return (
                  <div key={f.key} className="flex items-center gap-2">
                    <span className="w-32 shrink-0 text-xs text-muted">{f.label}</span>
                    <div className="h-2 flex-1 overflow-hidden rounded-full bg-base">
                      <div className="h-full rounded-full bg-accent" style={{ width: `${pct}%` }} />
                    </div>
                    <span className="tnum w-14 shrink-0 text-right text-xs text-ink">
                      {v}/{f.max}
                    </span>
                  </div>
                );
              })}
            </div>
          ) : (
            <p className="text-sm text-muted">Belum ada skor rule.</p>
          )}
          <p className="mt-3 text-xs leading-relaxed text-muted">
            {rationale(horizon, factors)}
          </p>
        </div>

        {/* AI side */}
        <div className="rounded-md border border-edge bg-surface/60 p-3">
          {ai?.verdict ? (
            <div>
              <div className="flex items-center justify-between gap-2">
                <span className="text-[10px] font-semibold uppercase tracking-wide text-muted">
                  AI Hermes
                </span>
                <div className="flex items-center gap-2">
                  {ai.confidence ? (
                    <span className="tnum text-xs text-muted">
                      keyakinan {formatPct(ai.confidence * 100)}
                    </span>
                  ) : null}
                  <VerdictBadge verdict={ai.verdict} size="md" />
                </div>
              </div>
              {ai.explanation && (
                <p className="mt-2 text-xs leading-relaxed text-ink whitespace-pre-line">
                  {ai.explanation}
                </p>
              )}
              {ai.risk_factors && ai.risk_factors.length > 0 && (
                <div className="mt-2 flex flex-wrap gap-1.5">
                  {ai.risk_factors.map((r, i) => (
                    <span
                      key={i}
                      className="rounded-md border border-danger/40 bg-danger/10 px-2 py-0.5 text-[11px] font-medium text-danger"
                    >
                      ⚠ {r}
                    </span>
                  ))}
                </div>
              )}
              {ai.updated_at && (
                <p className="tnum mt-2 text-[10px] text-muted">
                  Dianalisis {formatDateTime(ai.updated_at)}
                </p>
              )}
            </div>
          ) : (
            <p className="text-xs text-muted">
              {analyzing
                ? "Hermes sedang menganalisis data..."
                : "Belum ada analisis AI. Tekan “Analisis AI”."}
            </p>
          )}
        </div>
      </div>

      {/* Rule risk flags */}
      {horizon.risk_flags.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1.5">
          {horizon.risk_flags.map((flag) => (
            <span
              key={flag}
              className="rounded-md border border-danger/40 bg-danger/10 px-2 py-0.5 text-[11px] font-medium text-danger"
            >
              ⚠ {RISK_LABELS[flag] ?? flag}
            </span>
          ))}
        </div>
      )}
    </section>
  );
}
