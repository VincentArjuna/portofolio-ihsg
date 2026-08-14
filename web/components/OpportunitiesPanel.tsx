"use client";

import { useCallback, useEffect, useState } from "react";
import {
  fetchOpportunities,
  lookupTicker,
  refreshOpportunities,
  type LookupResult,
  type Opportunity,
} from "@/lib/api";
import { formatIDR } from "@/lib/format";
import VerdictBadge from "@/components/VerdictBadge";
import IndexBadge from "@/components/IndexBadge";
import StockDetailModal from "@/components/StockDetailModal";

type FilterKey = "" | "lq45" | "kompas100" | "both";

const FILTERS: { key: FilterKey; label: string }[] = [
  { key: "", label: "Semua" },
  { key: "lq45", label: "LQ45" },
  { key: "kompas100", label: "Kompas100" },
  { key: "both", label: "Keduanya" },
];

const COLS = [
  "Ticker",
  "Sektor",
  "Indeks",
  "Harga",
  "Verdict ST",
  "Verdict LT",
  "ROE",
  "PER",
  "Aksi",
];

function ScoreCell({ verdict, score }: { verdict: string; score: number }) {
  return (
    <div className="flex items-center gap-1.5">
      <VerdictBadge verdict={verdict} />
      <span className="tnum text-xs text-muted">{score}</span>
    </div>
  );
}

function SkeletonRow() {
  return (
    <tr className="border-t border-edge">
      {Array.from({ length: COLS.length }).map((_, i) => (
        <td key={i} className="px-4 py-3">
          <div className="skeleton h-4 w-full max-w-[5rem] rounded" />
        </td>
      ))}
    </tr>
  );
}

export default function OpportunitiesPanel() {
  const [opps, setOpps] = useState<Opportunity[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState<FilterKey>("");
  const [onlyBuy, setOnlyBuy] = useState(false);
  const [query, setQuery] = useState("");
  const [lookup, setLookup] = useState<LookupResult | null>(null);
  const [detailTicker, setDetailTicker] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetchOpportunities(filter, onlyBuy ? "BUY" : "");
      setOpps(res.opportunities ?? []);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memuat peluang");
    } finally {
      setLoading(false);
    }
  }, [filter, onlyBuy]);

  useEffect(() => {
    load();
  }, [load]);

  const refresh = useCallback(async () => {
    setRefreshing(true);
    setError(null);
    try {
      await refreshOpportunities();
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memperbarui peluang");
    } finally {
      setRefreshing(false);
    }
  }, [load]);

  const onSearch = useCallback(async () => {
    const q = query.trim();
    if (!q) return;
    setError(null);
    try {
      setLookup(await lookupTicker(q));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal mencari ticker");
    }
  }, [query]);

  const openDetail = (ticker: string) => setDetailTicker(ticker);

  const isEmpty = !loading && opps.length === 0;

  return (
    <section>
      {/* Filter + refresh bar */}
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-1.5">
          {FILTERS.map((f) => (
            <button
              key={f.key || "all"}
              type="button"
              onClick={() => setFilter(f.key)}
              className={`rounded-md border px-3 py-1.5 text-xs font-semibold transition-colors ${
                filter === f.key
                  ? "border-accent bg-accent/10 text-accent"
                  : "border-edge text-muted hover:border-accent hover:text-accent"
              }`}
            >
              {f.label}
            </button>
          ))}
          <label className="ml-2 flex cursor-pointer select-none items-center gap-1.5 text-xs text-muted">
            <input
              type="checkbox"
              checked={onlyBuy}
              onChange={(e) => setOnlyBuy(e.target.checked)}
              className="accent-accent"
            />
            Hanya Beli (ST)
          </label>
        </div>
        <button
          type="button"
          onClick={refresh}
          disabled={refreshing}
          className="rounded-md border border-edge bg-surface-1 px-4 py-2 text-sm font-medium text-secondary transition-colors hover:bg-surface-2 hover:text-ink disabled:cursor-not-allowed disabled:opacity-60"
        >
          {refreshing ? "Memperbarui..." : "Perbarui Peluang"}
        </button>
      </div>

      {/* Custom ticker search */}
      <div className="mb-4 rounded-lg border border-edge bg-surface-1 p-3">
        <div className="flex flex-wrap items-center gap-2">
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value.toUpperCase())}
            onKeyDown={(e) => e.key === "Enter" && onSearch()}
            placeholder="Cari ticker (mis. BBCA, atau ticker di luar LQ45/Kompas100)"
            className="tnum w-64 rounded-md border border-edge bg-surface-1 px-3 py-1.5 text-sm text-ink placeholder:text-muted/70 focus:border-accent focus:outline-none"
          />
          <button
            type="button"
            onClick={onSearch}
            className="rounded-md border border-accent/50 px-3 py-1.5 text-xs font-semibold text-accent transition-colors hover:bg-accent/10"
          >
            Cari
          </button>
          {lookup && (
            <span className="flex flex-wrap items-center gap-2 text-xs">
              <IndexBadge membership={lookup.index_membership} />
              {lookup.illiquid ? (
                <span className="rounded-md border border-danger/40 bg-danger/10 px-2 py-0.5 font-semibold text-danger">
                  ⚠ Illiquid — di luar LQ45/Kompas100
                </span>
              ) : (
                <span className="text-muted">
                  {lookup.sector ? `${lookup.sector} · ` : ""}Cair
                </span>
              )}
              <button
                type="button"
                onClick={() => openDetail(lookup.ticker)}
                className="rounded-md border border-edge px-2.5 py-1 text-xs font-medium text-muted transition-colors hover:border-accent hover:text-accent"
              >
                Buka Detail
              </button>
            </span>
          )}
        </div>
        <p className="mt-2 text-[11px] text-muted">
          Ticker di luar LQ45/Kompas100 ditandai <span className="text-danger">illiquid</span> sesuai profil Balanced-Growth.
        </p>
      </div>

      {error && (
        <div className="mb-4 rounded-md border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger">
          {error}
        </div>
      )}

      {/* Ranked opportunities table */}
      {isEmpty ? (
        <div className="rounded-lg border border-dashed border-edge bg-surface-1 p-12 text-center">
          <p className="text-base text-ink">Belum ada peluang yang ter-score.</p>
          <p className="mt-1 text-sm text-muted">
            Tekan “Perbarui Peluang” untuk memuat data LQ45/Kompas100.
          </p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-edge">
          <table className="w-full min-w-[960px] border-collapse text-sm">
            <thead className="bg-surface-1">
              <tr>
                {COLS.map((c) => (
                  <th
                    key={c}
                    className="px-4 py-3 text-left text-xs font-medium uppercase tracking-[0.05em] text-muted"
                  >
                    {c}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {loading ? (
                Array.from({ length: 5 }).map((_, i) => <SkeletonRow key={i} />)
              ) : (
                opps.map((o) => (
                  <tr
                    key={o.ticker}
                    className="border-t border-edge transition-colors hover:bg-surface-2"
                  >
                    <td className="px-4 py-3">
                      <div className="font-mono text-[14px] font-medium tracking-tight text-ink">
                        {o.ticker}
                      </div>
                      <div className="max-w-[14rem] truncate text-xs text-muted">
                        {o.company_name}
                      </div>
                    </td>
                    <td className="px-4 py-3 text-muted">{o.sector}</td>
                    <td className="px-4 py-3">
                      <IndexBadge membership={o.index_membership} />
                    </td>
                    <td className="tnum px-4 py-3 text-ink">
                      {formatIDR(o.last_price)}
                    </td>
                    <td className="px-4 py-3">
                      <ScoreCell verdict={o.short_term_rule} score={o.short_term_score} />
                    </td>
                    <td className="px-4 py-3">
                      <ScoreCell verdict={o.long_term_rule} score={o.long_term_score} />
                    </td>
                    <td className="tnum px-4 py-3 text-muted">
                      {o.roe ? `${o.roe.toFixed(1)}%` : "—"}
                    </td>
                    <td className="tnum px-4 py-3 text-muted">
                      {o.per ? o.per.toFixed(1) : "—"}
                    </td>
                    <td className="px-4 py-3">
                      <button
                        type="button"
                        onClick={() => openDetail(o.ticker)}
                        className="rounded-md border border-edge px-2.5 py-1 text-xs font-medium text-muted transition-colors hover:border-accent hover:text-accent"
                      >
                        Detail
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}

      <p className="mt-4 text-xs text-muted">
        Diurutkan Buy lebih dulu berdasarkan verdict jangka pendek lalu skor.
        Saham yang sudah Anda pegang tidak ditampilkan di daftar peluang.
      </p>

      <StockDetailModal
        ticker={detailTicker}
        onClose={() => setDetailTicker(null)}
      />
    </section>
  );
}
