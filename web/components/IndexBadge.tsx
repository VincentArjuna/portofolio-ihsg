// Index-membership badge for the LQ45/Kompas100 universe. Full class strings so
// Tailwind's scanner keeps them (no dynamic interpolation). Stays within the
// docs/DESIGN.md palette: accent (sky) for index membership, neutral edge for
// Kompas100-only — deliberately distinct from the green/yellow/red verdict scale.

const MAP: Record<string, { label: string; cls: string }> = {
  LQ45: { label: "LQ45", cls: "border-accent/50 bg-accent/10 text-accent" },
  KOMPAS100: {
    label: "Kompas100",
    cls: "border-edge bg-surface text-muted",
  },
  BOTH: {
    label: "LQ45 + Kompas100",
    cls: "border-accent/50 bg-accent/10 text-accent",
  },
};

export default function IndexBadge({
  membership,
}: {
  membership: string | null | undefined;
}) {
  if (!membership) {
    return (
      <span className="tnum rounded-md bg-base/60 px-2 py-0.5 text-xs text-muted">
        —
      </span>
    );
  }
  const m = MAP[membership] ?? {
    label: membership,
    cls: "border-edge bg-surface text-muted",
  };
  return (
    <span
      className={`tnum inline-flex items-center rounded-md border px-2 py-0.5 text-[11px] font-semibold ${m.cls}`}
    >
      {m.label}
    </span>
  );
}
