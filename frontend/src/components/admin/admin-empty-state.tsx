"use client";

type AdminEmptyStateProps = {
  title: string;
  description?: string;
  action?: React.ReactNode;
};

export function AdminEmptyState({ title, description, action }: AdminEmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center rounded-2xl border border-dashed border-border/80 bg-card/40 px-6 py-14 text-center">
      <p className="font-display text-lg font-bold">{title}</p>
      {description ? (
        <p className="mt-2 max-w-md text-sm text-muted-foreground">{description}</p>
      ) : null}
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  );
}
