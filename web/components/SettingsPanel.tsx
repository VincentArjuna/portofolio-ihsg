"use client";

import { useEffect, useState } from "react";
import { fetchSettings, saveSettings, type Settings } from "@/lib/api";
import { formatDateTime } from "@/lib/format";

interface Props {
  open: boolean;
  onClose: () => void;
}

// Preset intervals exposed in the UI (PRD: daily default; 12h/6h/hourly).
const INTERVALS = [
  { hours: 24, label: "Harian (24 jam)" },
  { hours: 12, label: "12 jam" },
  { hours: 6, label: "6 jam" },
  { hours: 1, label: "Per jam" },
];

export default function SettingsPanel({ open, onClose }: Props) {
  const [last, setLast] = useState<Settings | null>(null);
  const [enabled, setEnabled] = useState(false);
  const [hours, setHours] = useState(24);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  // Load current settings whenever the panel opens.
  useEffect(() => {
    if (!open) return;
    setError(null);
    setSaved(false);
    fetchSettings()
      .then((s) => {
        setLast(s);
        setEnabled(s.background_refresh_enabled);
        setHours(s.refresh_interval_hours);
      })
      .catch((e) =>
        setError(e instanceof Error ? e.message : "Gagal memuat pengaturan"),
      );
  }, [open]);

  // Close on Escape.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      const s = await saveSettings({
        background_refresh_enabled: enabled,
        refresh_interval_hours: hours,
      });
      setLast(s);
      setSaved(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menyimpan pengaturan");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="settings-title"
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-md rounded-lg border border-edge bg-surface p-6 shadow-xl"
      >
        <h2 id="settings-title" className="text-lg font-semibold text-ink">
          Pengaturan
        </h2>
        <p className="mt-1 text-sm text-muted">
          Atur pembaruan data pasar otomatis di latar belakang
        </p>

        <form onSubmit={submit} className="mt-5 space-y-5">
          <label className="flex cursor-pointer items-center justify-between">
            <span className="text-sm font-medium text-ink">
              Pembaruan otomatis
            </span>
            <input
              type="checkbox"
              role="switch"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="h-5 w-5 cursor-pointer rounded border-edge accent-accent"
            />
          </label>

          <div className={enabled ? "" : "pointer-events-none opacity-50"}>
            <label className="mb-1.5 block text-xs font-medium uppercase tracking-wide text-muted">
              Interval pembaruan
            </label>
            <select
              value={hours}
              onChange={(e) => setHours(Number(e.target.value))}
              className="w-full rounded-md border border-edge bg-base px-3 py-2 text-sm text-ink outline-none transition-colors focus:border-accent"
            >
              {INTERVALS.map((o) => (
                <option key={o.hours} value={o.hours}>
                  {o.label}
                </option>
              ))}
            </select>
          </div>

          {last?.last_background_refresh && (
            <p className="tnum text-xs text-muted">
              Pembaruan otomatis terakhir:{" "}
              {formatDateTime(last.last_background_refresh)}
            </p>
          )}

          {error && (
            <p className="rounded-md border border-danger/40 bg-danger/10 px-3 py-2 text-sm text-danger">
              {error}
            </p>
          )}
          {saved && !error && (
            <p className="rounded-md border border-success/40 bg-success/10 px-3 py-2 text-sm text-success">
              Pengaturan disimpan.
            </p>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border border-edge px-4 py-2 text-sm font-medium text-muted transition-colors hover:text-ink"
            >
              Tutup
            </button>
            <button
              type="submit"
              disabled={busy}
              className="rounded-md bg-accent px-4 py-2 text-sm font-semibold text-base transition-colors hover:brightness-110 disabled:opacity-60"
            >
              {busy ? "Menyimpan..." : "Simpan"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
