"use client";

type AuditDiffProps = {
  before: unknown;
  after: unknown;
  beforeLabel: string;
  afterLabel: string;
};

function pretty(v: unknown): string {
  if (v == null) return "—";
  try {
    return JSON.stringify(v, null, 2);
  } catch {
    return String(v);
  }
}

export function AuditDiff({ before, after, beforeLabel, afterLabel }: AuditDiffProps) {
  return (
    <div className="grid gap-2 sm:grid-cols-2">
      <div>
        <p className="mb-1 text-[10px] font-extrabold uppercase tracking-wider text-muted-foreground">
          {beforeLabel}
        </p>
        <pre className="max-h-48 overflow-auto rounded-xl border border-border/70 bg-background/60 p-2 font-mono text-[11px] text-muted-foreground">
          {pretty(before)}
        </pre>
      </div>
      <div>
        <p className="mb-1 text-[10px] font-extrabold uppercase tracking-wider text-muted-foreground">
          {afterLabel}
        </p>
        <pre className="max-h-48 overflow-auto rounded-xl border border-accent/30 bg-accent/[0.06] p-2 font-mono text-[11px]">
          {pretty(after)}
        </pre>
      </div>
    </div>
  );
}
