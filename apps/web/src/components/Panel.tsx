import type { ReactNode } from "react";

interface PanelProps {
  title: string;
  className?: string;
  // Query-ish state flags so each panel renders loading / error / empty
  // consistently (task 4.6). `isEmpty` is the "loaded but no data" case.
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
  isEmpty?: boolean;
  emptyHint?: string;
  children?: ReactNode;
}

export function Panel({
  title,
  className = "",
  isLoading,
  isError,
  error,
  isEmpty,
  emptyHint = "No data yet",
  children,
}: PanelProps) {
  return (
    <section
      className={`flex flex-col rounded-xl border border-ink-600/60 bg-ink-800/70 p-4 shadow-lg shadow-black/20 ${className}`}
    >
      <h2 className="mb-3 text-xs font-semibold uppercase tracking-widest text-slate-400">
        {title}
      </h2>
      <div className="flex-1">
        {isLoading ? (
          <PanelMessage>Loading…</PanelMessage>
        ) : isError ? (
          <PanelMessage tone="danger">
            Couldn’t load{errorCode(error) ? ` (${errorCode(error)})` : ""}
          </PanelMessage>
        ) : isEmpty ? (
          <PanelMessage>{emptyHint}</PanelMessage>
        ) : (
          children
        )}
      </div>
    </section>
  );
}

function PanelMessage({
  children,
  tone = "muted",
}: {
  children: ReactNode;
  tone?: "muted" | "danger";
}) {
  const color = tone === "danger" ? "text-accent-danger" : "text-slate-500";
  return (
    <div className={`flex h-full min-h-16 items-center justify-center text-sm ${color}`}>
      {children}
    </div>
  );
}

function errorCode(error: unknown): string | null {
  if (error && typeof error === "object" && "code" in error) {
    const code = (error as { code?: unknown }).code;
    if (typeof code === "string") return code;
  }
  return null;
}
