"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { Mail } from "lucide-react";
import { apiGet, apiPost } from "@/lib/api-client";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";

type InviteView = {
  id: string;
  token: string;
  org_id: string;
  org_name: string;
  role: string;
  expires_at: string;
};

export function OrgInvitesCard() {
  const t = useTranslations("Teacher");
  const [invites, setInvites] = useState<InviteView[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function load() {
    try {
      const rows = await apiGet<InviteView[]>("me/invites");
      setInvites(rows);
      setError(null);
    } catch {
      setInvites([]);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  async function accept(token: string) {
    setBusy(true);
    setError(null);
    try {
      await apiPost("me/invites/accept", { token });
      await load();
    } catch {
      setError(t("errorAccept"));
    } finally {
      setBusy(false);
    }
  }

  if (!invites || invites.length === 0) return null;

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Mail className="h-4 w-4" aria-hidden />
          {t("invitesCardTitle")}
        </CardTitle>
      </CardHeader>
      <div className="space-y-3 px-6 pb-6">
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
        <ul className="space-y-2">
          {invites.map((inv) => (
            <li key={inv.id} className="flex items-center justify-between gap-2 text-sm">
              <div>
                <p className="font-semibold">{inv.org_name}</p>
                <p className="text-xs text-muted-foreground">{inv.role}</p>
              </div>
              <Button type="button" size="sm" disabled={busy} onClick={() => void accept(inv.token)}>
                {t("acceptInvite")}
              </Button>
            </li>
          ))}
        </ul>
      </div>
    </Card>
  );
}
