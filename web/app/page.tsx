"use client";

import { useCallback, useEffect, useState } from "react";
import {
  createPosition,
  deletePosition,
  fetchPortfolio,
  updatePosition,
  type Portfolio,
  type Position,
  type PositionInput,
} from "@/lib/api";
import KpiCards from "@/components/KpiCards";
import HoldingsTable from "@/components/HoldingsTable";
import PositionModal from "@/components/PositionModal";

export default function Page() {
  const [data, setData] = useState<Portfolio | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<Position | null>(null);

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
        <button
          type="button"
          onClick={openAdd}
          className="rounded-md bg-accent px-4 py-2 text-sm font-semibold text-base transition-colors hover:brightness-110"
        >
          + Tambah Posisi
        </button>
      </header>

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
        />
      )}

      {/* Disagreement / data-freshness footnote */}
      <p className="mt-4 text-xs text-muted">
        Data verdict (Aturan vs AI), harga kini, dan P&amp;L akan hadir setelah
        integrasi data pasar di T2.
      </p>

      <PositionModal
        open={modalOpen}
        initial={editing}
        onClose={() => setModalOpen(false)}
        onSubmit={handleSubmit}
      />
    </main>
  );
}
