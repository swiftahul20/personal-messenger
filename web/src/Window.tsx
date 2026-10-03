import type { ReactNode } from "react";

interface WindowProps {
  title: string;
  status?: string;
  actions?: ReactNode;
  label: string;
  className?: string;
  children: ReactNode;
}

export function Window({
  title,
  status,
  actions,
  label,
  className = "",
  children,
}: WindowProps) {
  return (
    <section
      aria-label={label}
      className={`flex min-h-0 flex-col overflow-hidden rounded-win border border-brand bg-white shadow-window ${className}`}
    >
      <header className="flex items-center gap-3 bg-linear-to-b from-title-light to-title-dark px-3 py-2 text-white">
        <h2 className="min-w-0 flex-1 truncate text-sm font-bold">
          {title}
          {status ? <span className="ml-2 font-normal">{status}</span> : null}
        </h2>
        {actions}
      </header>
      {children}
    </section>
  );
}

export const titleButton =
  "min-h-11 rounded-win border border-white px-3 text-sm text-white hover:bg-white/15 focus-visible:outline-white sm:min-h-8";

export const primaryButton =
  "min-h-11 rounded-win border border-ink bg-accent px-4 text-sm font-bold text-ink hover:brightness-95 disabled:opacity-60 sm:min-h-9";

export const secondaryButton =
  "min-h-11 rounded-win border border-line bg-white px-3 text-sm text-ink hover:bg-selected disabled:opacity-60 sm:min-h-9";

export const inputClass =
  "min-h-11 w-full min-w-0 rounded-win border border-line bg-white px-2 text-sm text-ink sm:min-h-9";
