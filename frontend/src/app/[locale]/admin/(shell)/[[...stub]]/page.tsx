export default function AdminStubOrOverviewPage({
  params,
}: {
  params: { stub?: string[] };
}) {
  const parts = params.stub ?? [];
  if (parts.length === 0) {
    return (
      <main className="mx-auto max-w-4xl space-y-4">
        <h1 className="font-display text-2xl font-extrabold tracking-tight">Overview</h1>
        <p className="text-sm leading-6 text-muted-foreground">
          Driver Go Super Admin control center. Users, Content va Payments live; boshqa sidebar
          modullar M3 bosqichlarida. Contacts hozircha Ops bridge orqali:{" "}
          <code className="rounded bg-card px-1.5 py-0.5 text-xs">/ops/contacts</code> — keyin Admin
          CMS chrome.
        </p>
        <div className="grid gap-3 sm:grid-cols-3">
          {[
            { label: "Users", hint: "M3-1 ✓" },
            { label: "Content", hint: "M3-2 ✓" },
            { label: "Payments", hint: "M3-3 ✓" },
            { label: "CMS", hint: "M3-4" },
            { label: "Monitoring", hint: "M3-5" },
            { label: "Security", hint: "M3-0+" },
          ].map((tile) => (
            <div key={tile.label} className="rounded-xl border border-border bg-card px-4 py-3">
              <p className="text-sm font-bold">{tile.label}</p>
              <p className="text-xs text-muted-foreground">{tile.hint}</p>
            </div>
          ))}
        </div>
      </main>
    );
  }

  return (
    <main className="mx-auto max-w-2xl space-y-3">
      <h1 className="font-display text-xl font-extrabold">Tez orada</h1>
      <p className="text-sm text-muted-foreground">
        Bo‘lim: <code className="rounded bg-card px-1.5 py-0.5 text-xs">{parts.join("/")}</code>. M3
        rejasiga ko‘ra keyingi bosqichda to‘ldiriladi.
      </p>
    </main>
  );
}
