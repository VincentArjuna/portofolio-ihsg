"use client";

import { useEffect, useRef, useState } from "react";
import type { Position, PositionInput } from "@/lib/api";

interface Props {
  open: boolean;
  /** When set, the modal edits this position; otherwise it creates a new one. */
  initial: Position | null;
  onClose: () => void;
  onSubmit: (input: PositionInput, id: string | null) => Promise<void>;
}

const today = () => new Date().toISOString().slice(0, 10);

const empty: PositionInput = { ticker: "", shares: 0, avg_buy_price: 0, buy_date: today() };

export default function PositionModal({ open, initial, onClose, onSubmit }: Props) {
  const [form, setForm] = useState<PositionInput>(empty);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const firstField = useRef<HTMLInputElement>(null);

  // Reset form whenever the modal opens (for add or edit).
  useEffect(() => {
    if (!open) return;
    setError(null);
    setForm(
      initial
        ? {
            ticker: initial.ticker,
            shares: initial.shares,
            avg_buy_price: initial.avg_buy_price,
            buy_date: initial.buy_date,
          }
        : empty,
    );
    setTimeout(() => firstField.current?.focus(), 10);
  }, [open, initial]);

  // Close on Escape.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  const set = <K extends keyof PositionInput>(key: K, value: PositionInput[K]) =>
    setForm((f) => ({ ...f, [key]: value }));

  const validate = (): string | null => {
    if (!form.ticker.trim()) return "Ticker wajib diisi";
    if (form.shares <= 0) return "Jumlah saham harus lebih dari 0";
    if (form.avg_buy_price <= 0) return "Harga beli rata-rata harus lebih dari 0";
    if (!/^\d{4}-\d{2}-\d{2}$/.test(form.buy_date)) return "Tanggal beli tidak valid";
    return null;
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const err = validate();
    if (err) return setError(err);
    setBusy(true);
    setError(null);
    try {
      await onSubmit(
        { ...form, ticker: form.ticker.trim().toUpperCase() },
        initial?.id ?? null,
      );
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Terjadi kesalahan");
    } finally {
      setBusy(false);
    }
  };

  const field =
    "w-full rounded-md border border-edge bg-surface-1 px-3 py-2 text-sm text-ink outline-none transition-colors focus:border-accent";

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/85 p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-md rounded-xl border border-edge bg-surface p-6 shadow-xl"
      >
        <h2 id="modal-title" className="text-lg font-semibold text-ink">
          {initial ? "Edit Posisi" : "Tambah Posisi"}
        </h2>
        <p className="mt-1 text-sm text-muted">
          {initial
            ? `Perbarui data posisi ${initial.ticker}`
            : "Masukkan data saham IHSG yang Anda miliki"}
        </p>

        <form onSubmit={submit} className="mt-5 space-y-4">
          <div>
            <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-muted">
              Ticker
            </label>
            <input
              ref={firstField}
              type="text"
              value={form.ticker}
              onChange={(e) => set("ticker", e.target.value.toUpperCase())}
              placeholder="BBCA"
              className={`${field} uppercase`}
              maxLength={8}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-muted">
                Jumlah Saham
              </label>
              <input
                type="number"
                min="0.0001"
                step="0.0001"
                value={form.shares === 0 ? "" : form.shares}
                onChange={(e) => set("shares", Number(e.target.value))}
                placeholder="1000"
                className={field}
              />
            </div>
            <div>
              <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-muted">
                Harga Beli (IDR)
              </label>
              <input
                type="number"
                min="0.01"
                step="0.01"
                value={form.avg_buy_price === 0 ? "" : form.avg_buy_price}
                onChange={(e) => set("avg_buy_price", Number(e.target.value))}
                placeholder="6000"
                className={field}
              />
            </div>
          </div>
          <div>
            <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-muted">
              Tanggal Beli
            </label>
            <input
              type="date"
              value={form.buy_date}
              onChange={(e) => set("buy_date", e.target.value)}
              className={field}
            />
          </div>

          {error && (
            <p className="rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">
              {error}
            </p>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border border-edge px-4 py-2 text-sm font-medium text-muted transition-colors hover:text-ink"
            >
              Batal
            </button>
            <button
              type="submit"
              disabled={busy}
              className="rounded-md bg-accent px-4 py-2 text-sm font-medium text-primary transition-colors hover:bg-accent-hover disabled:opacity-60"
            >
              {busy ? "Menyimpan..." : initial ? "Simpan Perubahan" : "Tambah"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
