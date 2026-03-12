"use client";

import { useEffect, useState, type ReactNode } from "react";

export type Tone = "slate" | "cyan" | "emerald" | "amber" | "rose" | "violet";

type PageHeroProps = {
  eyebrow: string;
  title: string;
  description: string;
  badges?: ReactNode;
  actions?: ReactNode;
};

type PanelProps = {
  title: string;
  description?: string;
  badge?: ReactNode;
  children: ReactNode;
  className?: string;
};

type MetricCardProps = {
  label: string;
  value: ReactNode;
  description?: ReactNode;
  tone?: Tone;
};

export function cx(...classes: Array<string | false | null | undefined>): string {
  return classes.filter(Boolean).join(" ");
}

function toneSurfaceClasses(tone: Tone): string {
  switch (tone) {
    case "cyan":
      return "border-cyan-200 bg-cyan-50 text-cyan-800";
    case "emerald":
      return "border-emerald-200 bg-emerald-50 text-emerald-800";
    case "amber":
      return "border-amber-200 bg-amber-50 text-amber-800";
    case "rose":
      return "border-rose-200 bg-rose-50 text-rose-800";
    case "violet":
      return "border-violet-200 bg-violet-50 text-violet-800";
    default:
      return "border-gray-200 bg-gray-50 text-gray-700";
  }
}

function toneTextClasses(tone: Tone): string {
  switch (tone) {
    case "cyan":
      return "text-cyan-700";
    case "emerald":
      return "text-emerald-700";
    case "amber":
      return "text-amber-700";
    case "rose":
      return "text-rose-700";
    case "violet":
      return "text-violet-700";
    default:
      return "text-gray-600";
  }
}

export function ReportShell({ children }: { children: ReactNode }) {
  return (
    <div className="space-y-6 px-0">
      <div className="w-full space-y-6">{children}</div>
    </div>
  );
}

export function StatusPill({
  children,
  tone = "slate",
  className,
}: {
  children: ReactNode;
  tone?: Tone;
  className?: string;
}) {
  return (
    <span
      className={cx(
        "inline-flex items-center rounded-md border px-2.5 py-1 text-xs font-medium",
        toneSurfaceClasses(tone),
        className
      )}
    >
      {children}
    </span>
  );
}

export function PageHero({ eyebrow, title, description, badges, actions }: PageHeroProps) {
  return (
    <section className="rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="space-y-3">
          <div className="text-xs font-semibold uppercase tracking-[0.16em] text-blue-600">{eyebrow}</div>
          <div className="space-y-2">
            <h1 className="text-2xl font-bold text-gray-900">{title}</h1>
            <p className="max-w-3xl text-sm leading-6 text-gray-600">{description}</p>
          </div>
          {badges ? <div className="flex flex-wrap gap-2">{badges}</div> : null}
        </div>
        {actions ? <div className="flex flex-wrap gap-2 lg:justify-end">{actions}</div> : null}
      </div>
    </section>
  );
}

export function Panel({ title, description, badge, children, className }: PanelProps) {
  return (
    <section className={cx("rounded-lg border border-gray-200 bg-white p-6 shadow-sm", className)}>
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
          {description ? <p className="mt-1 text-sm text-gray-500">{description}</p> : null}
        </div>
        {badge}
      </div>
      {children}
    </section>
  );
}

export function MetricCard({ label, value, description, tone = "slate" }: MetricCardProps) {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-4 shadow-sm">
      <div className={cx("text-xs font-semibold uppercase tracking-[0.14em]", toneTextClasses(tone))}>{label}</div>
      <div className="mt-2 text-xl font-semibold text-gray-900">{value}</div>
      {description ? <div className="mt-2 text-sm text-gray-500">{description}</div> : null}
    </div>
  );
}

export function EmptyNotice({
  message,
  tone = "slate",
}: {
  message: string;
  tone?: Tone;
}) {
  return (
    <div className={cx("rounded-lg border px-4 py-3 text-sm", toneSurfaceClasses(tone))}>
      {message}
    </div>
  );
}

function formatKSTClock(date: Date): string {
  return new Intl.DateTimeFormat("ko-KR", {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
    timeZone: "Asia/Seoul",
  }).format(date);
}

export function KSTClockPill({
  prefix = "KST",
  tone = "slate",
}: {
  prefix?: string;
  tone?: Tone;
}) {
  const [clock, setClock] = useState<string>("--:--:--");

  useEffect(() => {
    const updateClock = () => {
      setClock(formatKSTClock(new Date()));
    };

    updateClock();
    const timer = window.setInterval(updateClock, 1000);
    return () => window.clearInterval(timer);
  }, []);

  return <StatusPill tone={tone}>{prefix} {clock}</StatusPill>;
}

function progressWidth(remaining: number, total: number): string {
  if (total <= 0) {
    return "0%";
  }
  return `${((total - remaining) / total) * 100}%`;
}

export function RefreshCountdownCard({
  enabled,
  seconds,
  updatedAt,
  barClassName,
}: {
  enabled: boolean;
  seconds: number;
  updatedAt?: string;
  barClassName: string;
}) {
  const [remaining, setRemaining] = useState<number>(seconds);

  useEffect(() => {
    if (!enabled) {
      return;
    }
    const timer = window.setInterval(() => {
      setRemaining((prev) => {
        if (prev <= 1) {
          return seconds;
        }
        return prev - 1;
      });
    }, 1000);
    return () => window.clearInterval(timer);
  }, [enabled, seconds]);

  return (
    <div className="rounded-lg border border-slate-200 bg-slate-50 p-4">
      <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
        <span className="font-semibold text-slate-700">
          {enabled ? `${remaining}초 후 자동 갱신` : "자동 갱신 비활성화"}
        </span>
        <span className="text-slate-500">{updatedAt || "-"}</span>
      </div>
      <div className="mt-3 h-2 overflow-hidden rounded-full bg-slate-200">
        <div
          className={cx("h-full rounded-full transition-[width] duration-1000 linear", barClassName)}
          style={{ width: enabled ? progressWidth(remaining, seconds) : "0%" }}
        />
      </div>
    </div>
  );
}
