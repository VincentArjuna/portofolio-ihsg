"use client";

import { useCallback, useEffect, useState } from "react";
import {
  createPosition,
  deletePosition,
  fetchPortfolio,
  refreshMarketData,
  updatePosition,
  type Portfolio,
  type Position,
  type PositionInput,
} from "@/lib/api";
import { isStale } from "@/lib/format";
import KpiCards from "@/components/KpiCards";
import HoldingsTable from "@/components/HoldingsTable";
import PositionModal from "@/components/PositionModal";
import SettingsPanel from "@/components/SettingsPanel";
import StockDetailModal from "@/components/StockDetailModal";

export default function Page() {
  const [data, setData] = useState<Portfolio | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Position | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [detailTicker, setDetailTicker] = useState<string | null>(null);
  const [detailAutoAnalyze, setDetailAutoAnalyze] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setData(await fetchPortfolio());
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memuat data");
    } finally {
      setLoading(false);
    }
  }, []);

  // Market-data refresh: pull delayed quotes for held tickers, then reload
  // portfolio so P&L reflects the new prices.
  const refreshMarket = useCallback(async () => {
    setRefreshing(true);
    setError(null);
    try {
      await refreshMarketData();
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memperbarui data pasar");
    } finally {
      setRefreshing(false);
    }
  }, [refresh]);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const openAdd = () => {
    setEditing(null);
    setModalOpen(true);
  };
  const openEdit = (p: Position) => {
    setEditing(p);
    setModalOpen(true);
  };
  const openDetail = (p: Position) => {
    setDetailAutoAnalyze(false);
    setDetailTicker(p.ticker);
  };
  const openAnalyze = (p: Position) => {
    setDetailAutoAnalyze(true);
    setDetailTicker(p.ticker);
  };

  const handleSubmit = async (input: PositionInput, id: string | null) => {
    if (id) await updatePosition(id, input);
    else await createPosition(input);
    await refresh();
  };

  const handleDelete = async (p: Position) => {
    if (!window.confirm(`Hapus posisi ${p.ticker}? Tindakan ini tidak dapat dibatalkan.`))
      return;
    try {
      await deletePosition(p.id);
      await refresh();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menghapus posisi");
    }
  };

  const positions = data?.positions ?? [];
  const isEmpty = !loading && positions.length === 0;

  return (
    <main className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Top bar */}
      <header className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-ink">
            Portofolio IHSG
          </h1>
          <p className="mt-1 text-sm text-muted">
            Pantau kepemilikan saham dan alokasi modal Anda
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setSettingsOpen(true)}
            className="rounded-md border border-edge px-4 py-2 text-sm font-semibold text-ink transition-colors hover:border-accent hover:text-accent"
          >
            Pengaturan
          </button>
          <button
            type="button"
            onClick={refreshMarket}
            disabled={refreshing || positions.length === 0}
            className="rounded-md border border-edge px-4 py-2 text-sm font-semibold text-ink transition-colors hover:border-accent hover:text-accent disabled:cursor-not-allowed disabled:opacity-60"
          >
            {refreshing ? "Memperbarui..." : "Perbarui Data"}
          </button>
          <button
            type="button"
            onClick={openAdd}
            className="rounded-md bg-accent px-4 py-2 text-sm font-semibold text-base transition-colors hover:brightness-110"
          >
            + Tambah Posisi
          </button>
        </div>
      </header>

      {/* Stale-data banner */}
      {data?.summary.last_market_update &&
        isStale(data.summary.last_market_update) && (
          <div className="mb-4 rounded-md border border-warning/40 bg-warning/10 px-4 py-2.5 text-sm text-warning">
            Data pasar sudah basi — tekan “Perbarui Data” untuk memuat harga
            terbaru.
          </div>
        )}

      {/* KPI cards */}
      {data && (
        <section className="mb-6">
          <KpiCards summary={data.summary} positionCount={positions.length} />
        </section>
      )}

      {/* Error banner */}
      {error && (
        <div className="mb-4 rounded-md border border-danger/40 bg-danger/10 px-4 py-3 text-sm text-danger">
          {error}
        </div>
      )}

      {/* Holdings */}
      {isEmpty ? (
        <div className="rounded-lg border border-dashed border-edge bg-surface/40 p-12 text-center">
          <p className="text-base text-ink">Belum ada saham di portofolio.</p>
          <p className="mt-1 text-sm text-muted">Tambah saham pertama Anda.</p>
          <button
            type="button"
            onClick={openAdd}
            className="mt-5 rounded-md bg-accent px-4 py-2 text-sm font-semibold text-base transition-colors hover:brightness-110"
          >
            + Tambah Posisi
          </button>
        </div>
      ) : (
        <HoldingsTable
          positions={positions}
          loading={loading}
          onEdit={openEdit}
          onDelete={handleDelete}
          onDetail={openDetail}
          onAnalyze={openAnalyze}
        />
      )}

      {/* Disagreement / data-freshness footnote */}
      <p className="mt-4 text-xs text-muted">
        Harga kini dan P&amp;L memakai data tertunda (delayed) dari Yahoo
        Finance. Verdict rule (Beli/Tahan/Jual) dihitung deterministik untuk
        profil Balanced-Growth; sisi AI hadir di T4.
      </p>

      <PositionModal
        open={modalOpen}
        initial={editing}
        onClose={() => setModalOpen(false)}
        onSubmit={handleSubmit}
      />

      <SettingsPanel open={settingsOpen} onClose={() => setSettingsOpen(false)} />

      <StockDetailModal
        ticker={detailTicker}
        autoAnalyze={detailAutoAnalyze}
        onClose={() => setDetailTicker(null)}
      />
    </main>
  );
}
