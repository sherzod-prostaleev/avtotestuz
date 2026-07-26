"use client";

import Link from "next/link";

type MetricTileProps = {
  label: string;
  value: string;
  hint?: string;
  href?: string;
  sparkline?: number[];
};

function Spark({ values }: { values: number[] }) {
  if (values.length < 2) return null;
  const max = Math.max(1, ...values);
  const w = 64;
  const h = 20;
  const pts = values
    .map((v, i) => {
      const x = (i / (values.length - 1)) * w;
      const y = h - (v / max) * h;
      return `${x},${y}`;
    })
    .join(" ");
  return (
    <svg width={w} height={h} className="mt-2 opacity-80" aria-hidden>
      <polyline fill="none" stroke="hsl(var(--accent))" strokeWidth="1.5" points={pts} />
    </svg>
  );
}

export function MetricTile({ label, value, hint, href, sparkline }: MetricTileProps) {
  const body = (
    <>
      <p className="text-[11px] font-bold uppercase tracking-wider text-muted-foreground">
        {label}
      </p>
      <p className="mt-2 font-display text-2xl font-black tabular-nums tracking-tight">{value}</p>
      {hint ? <p className="mt-1 text-[11px] text-muted-foreground">{hint}</p> : null}
      {sparkline ? <Spark values={sparkline} /> : null}
    </>
  );

  const cls =
    "block rounded-2xl border border-border/80 bg-card/80 p-4 transition hover:border-accent/35";

  if (href) {
    return (
      <Link href={href} className={`${cls} hover:bg-accent/[0.05]`}>
        {body}
      </Link>
    );
  }
  return <div className={cls}>{body}</div>;
}
