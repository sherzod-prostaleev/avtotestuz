"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { ArrowLeft, Users } from "lucide-react";
import { apiGet } from "@/lib/api-client";

type OrgSummary = {
  id: string;
  name: string;
  status: string;
  my_role: string;
  member_count: number;
  active_seats: number;
};

type OrgDetail = {
  org: OrgSummary;
  members: { profile_id: string; phone_masked: string; name: string; role: string }[];
  licenses: { id: string; seats: number; ends_at: string; active: boolean; note: string }[];
};

export default function TeacherPage() {
  const t = useTranslations("Teacher");
  const locale = useLocale();
  const [orgs, setOrgs] = useState<OrgSummary[] | null>(null);
  const [selected, setSelected] = useState<OrgDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadOrgs = useCallback(async () => {
    setError(null);
    try {
      const data = await apiGet<OrgSummary[]>("me/teacher/orgs");
      setOrgs(data);
      if (data.length === 1) {
        const detail = await apiGet<OrgDetail>(`me/teacher/orgs/${data[0].id}`);
        setSelected(detail);
      }
    } catch {
      setError(t("errorLoad"));
      setOrgs([]);
    }
  }, [t]);

  useEffect(() => {
    void loadOrgs();
  }, [loadOrgs]);

  async function openOrg(id: string) {
    setError(null);
    try {
      const detail = await apiGet<OrgDetail>(`me/teacher/orgs/${id}`);
      setSelected(detail);
    } catch {
      setError(t("errorLoad"));
    }
  }

  return (
    <main className="mx-auto max-w-2xl space-y-4 px-4 py-8">
      <Link
        href={`/${locale}/profile`}
        className="inline-flex items-center gap-2 text-sm font-bold text-muted-foreground hover:text-foreground"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden /> {t("back")}
      </Link>
      <header>
        <h1 className="font-display text-2xl font-extrabold tracking-tight">{t("title")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("subtitle")}</p>
        <p className="mt-2 text-xs text-muted-foreground">{t("note")}</p>
      </header>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {!orgs ? (
        <p className="text-sm text-muted-foreground">{t("loading")}</p>
      ) : orgs.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t("empty")}</p>
      ) : (
        <ul className="space-y-2">
          {orgs.map((o) => (
            <li key={o.id}>
              <button
                type="button"
                onClick={() => void openOrg(o.id)}
                className={`w-full rounded-xl border px-4 py-3 text-left transition-colors ${
                  selected?.org.id === o.id
                    ? "border-accent bg-accent/10"
                    : "border-border bg-card hover:border-accent/40"
                }`}
              >
                <p className="font-semibold">{o.name}</p>
                <p className="text-xs text-muted-foreground">
                  {o.my_role} · {t("membersSeats", { members: o.member_count, seats: o.active_seats })}
                </p>
              </button>
            </li>
          ))}
        </ul>
      )}

      {selected ? (
        <section className="space-y-3 rounded-xl border border-border bg-card p-4">
          <div className="flex items-center gap-2">
            <Users className="h-4 w-4 text-muted-foreground" aria-hidden />
            <h2 className="text-sm font-bold">{t("membersTitle")}</h2>
          </div>
          <ul className="space-y-2">
            {selected.members.map((m) => (
              <li key={m.profile_id} className="text-sm">
                <p className="font-semibold">{m.name || m.phone_masked}</p>
                <p className="font-mono text-xs text-muted-foreground">
                  {m.phone_masked} · {m.role}
                </p>
              </li>
            ))}
          </ul>
          <div>
            <h3 className="mb-1 text-xs font-bold uppercase tracking-wider text-muted-foreground">
              {t("licensesTitle")}
            </h3>
            {selected.licenses.length === 0 ? (
              <p className="text-xs text-muted-foreground">{t("noLicenses")}</p>
            ) : (
              <ul className="space-y-1 text-xs font-mono text-muted-foreground">
                {selected.licenses.map((l) => (
                  <li key={l.id}>
                    {l.seats} seats · {l.active ? "active" : "ended"} ·{" "}
                    {new Date(l.ends_at).toLocaleDateString(locale)}
                  </li>
                ))}
              </ul>
            )}
          </div>
        </section>
      ) : null}
    </main>
  );
}
