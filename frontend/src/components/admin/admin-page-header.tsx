"use client";

type AdminPageHeaderProps = {
  badge?: string;
  title: string;
  description?: string;
  actions?: React.ReactNode;
};

export function AdminPageHeader({ badge, title, description, actions }: AdminPageHeaderProps) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-3">
      <div className="min-w-0">
        {badge ? (
          <p className="text-[11px] font-extrabold uppercase tracking-[0.18em] text-accent">
            {badge}
          </p>
        ) : null}
        <h1 className="mt-1 font-display text-3xl font-black tracking-tight">{title}</h1>
        {description ? (
          <p className="mt-1 max-w-2xl text-sm leading-6 text-muted-foreground">{description}</p>
        ) : null}
      </div>
      {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
    </header>
  );
}
