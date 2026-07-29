"use client";

export type AdminChartPoint = {
  label: string;
  value: number;
};

type AdminBarChartProps = {
  points: AdminChartPoint[];
  emptyLabel: string;
  valueFormatter?: (n: number) => string;
  height?: number;
};

/** Lightweight SVG bar chart — no chart-lib dependency. */
export function AdminBarChart({
  points,
  emptyLabel,
  valueFormatter = (n) => String(n),
  height = 160,
}: AdminBarChartProps) {
  // The API always returns a full 14-day window (generate_series), so "no
  // points" never happens and "every day zero" is the real empty case. Hiding
  // the chart at fewer than two populated days was worse than the ghosts it
  // removed: on a quiet week the panel claimed there was no data while the
  // revenue tile directly above it showed a non-zero sum.
  const populatedDays = points.filter((p) => p.value > 0).length;
  if (populatedDays === 0) {
    return (
      <p className="flex h-40 items-center justify-center rounded-xl border border-dashed border-border px-4 text-center text-sm text-muted-foreground">
        {emptyLabel}
      </p>
    );
  }

  const max = Math.max(1, ...points.map((p) => p.value));
  const padX = 8;
  const padTop = 16;
  const padBottom = 28;
  const barGap = 4;
  const width = Math.max(280, points.length * 28);
  const innerH = height - padTop - padBottom;
  const barW = (width - padX * 2 - barGap * (points.length - 1)) / points.length;

  return (
    <div className="w-full overflow-x-auto">
      <svg
        role="img"
        viewBox={`0 0 ${width} ${height}`}
        className="h-40 w-full min-w-[280px]"
        aria-label="chart"
      >
        <line
          x1={padX}
          y1={height - padBottom}
          x2={width - padX}
          y2={height - padBottom}
          className="stroke-border"
          strokeWidth={1}
        />
        {points.map((p, i) => {
          const h = (p.value / max) * innerH;
          const x = padX + i * (barW + barGap);
          const y = padTop + (innerH - h);
          const short =
            p.label.length > 6 ? p.label.slice(5) : p.label.slice(-2);
          return (
            <g key={`${p.label}-${i}`}>
              <title>
                {p.label}: {valueFormatter(p.value)}
              </title>
              <rect
                x={x}
                y={y}
                width={barW}
                height={Math.max(h, p.value > 0 ? 2 : 0)}
                rx={3}
                className="fill-accent"
                opacity={p.value > 0 ? 0.95 : 0.25}
              />
              <text
                x={x + barW / 2}
                y={height - 10}
                textAnchor="middle"
                className="fill-muted-foreground"
                fontSize={9}
              >
                {short}
              </text>
            </g>
          );
        })}
      </svg>
    </div>
  );
}
