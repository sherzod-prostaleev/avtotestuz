"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { apiGet, apiPatch } from "@/lib/api-client";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft, User, LogOut, Check } from "lucide-react";
import { ReferralCard } from "@/components/profile/referral-card";
import { PaymentHistoryCard } from "@/components/profile/payment-history-card";
import { TelegramLinkCard } from "@/components/profile/telegram-link-card";
import { ChangePasswordForm } from "@/components/profile/change-password-form";
import { ProfileMobile } from "@/components/profile/mobile/profile-mobile";

interface UserProfileData {
  id: string;
  phone: string;
  name: string;
  region: string;
  district: string;
  birth_date: string | null;
  locale_pref: string;
  theme_pref: string;
  referral_code: string;
  role: string;
  must_change_password?: boolean;
  created_at: string;
}

interface MeResponse {
  profile: UserProfileData;
  vip: { active: boolean; until: string | null };
}

export default function ProfilePage() {
  const t = useTranslations("Profile");
  const currentLocale = useLocale();
  const router = useRouter();

  const [profile, setProfile] = useState<UserProfileData | null>(null);
  const [name, setName] = useState<string>("");
  const [region, setRegion] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [saving, setSaving] = useState<boolean>(false);
  const [savedSuccess, setSavedSuccess] = useState<boolean>(false);
  const [errorKey, setErrorKey] = useState<"loadError" | "saveError" | null>(null);
  const [isVip, setIsVip] = useState<boolean>(false);

  const loadProfile = useCallback(async () => {
    setLoading(true);
    setErrorKey(null);
    try {
      const data = await apiGet<MeResponse>("me");
      setProfile(data.profile);
      setName(data.profile.name);
      setRegion(data.profile.region);
      setIsVip(data.vip.active);
    } catch {
      setErrorKey("loadError");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadProfile();
  }, [loadProfile]);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setErrorKey(null);
    setSavedSuccess(false);
    try {
      const updated = await apiPatch<UserProfileData>("me", { name, region });
      setProfile(updated);
      setSavedSuccess(true);
      setTimeout(() => setSavedSuccess(false), 3000);
    } catch {
      setErrorKey("saveError");
    } finally {
      setSaving(false);
    }
  };


  const handleLogout = async () => {
    try {
      await fetch("/api/auth/logout", { method: "POST" });
    } finally {
      if ("serviceWorker" in navigator) {
        const registration = await navigator.serviceWorker.getRegistration("/");
        registration?.active?.postMessage({ type: "CLEAR_PRIVATE_CACHES" });
      }
      router.push(`/${currentLocale}/login`);
    }
  };

  return (
    <main className="page-shell-narrow max-md:!pb-0">
      {/* The phone rebuilds the header inside the list — the user card carries
          the name, so a separate title and paragraph is a screen of chrome. */}
      <header className="mb-6 max-md:hidden">
        <Link href={`/${currentLocale}/dashboard`} className="back-link">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
        </Link>
        <h1 className="font-display text-2xl font-bold tracking-tight">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </header>

      <div className="space-y-6">
        {/* `md:[&+*]:!mt-0`: a `md:hidden` element still counts as a child for
            `space-y-6`, and would otherwise hand the first card below it a
            margin the wide layout never had. */}
        <ProfileMobile
          className="md:hidden md:[&+*]:!mt-0"
          name={name}
          region={region}
          phone={profile?.phone ?? ""}
          referralCode={profile?.referral_code ?? ""}
          isVip={isVip}
          onNameChange={setName}
          onRegionChange={setRegion}
          onSave={handleSave}
          saving={saving}
          saved={savedSuccess}
          errorKey={errorKey}
          onRetry={() => void loadProfile()}
          onLogout={() => void handleLogout()}
          loading={loading}
        />
        <Card className="p-5 max-md:hidden sm:p-6">
          <CardHeader className="mb-4 flex flex-row items-center gap-2 p-0">
            <User aria-hidden="true" className="h-5 w-5 text-accent" />
            <CardTitle className="text-base font-bold">{t("personalInfo")}</CardTitle>
          </CardHeader>

          {loading && (
            <div role="status" className="mb-4 text-sm text-muted-foreground">
              {t("loading")}
            </div>
          )}

          {errorKey && (
            <div role="alert" className="mb-4 rounded-xl border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
              <p>{t(errorKey)}</p>
              {errorKey === "loadError" && (
                <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void loadProfile()}>
                  {t("retry")}
                </Button>
              )}
            </div>
          )}

          {savedSuccess && (
            <div role="status" className="mb-4 flex items-center gap-2 rounded-xl border border-success/50 bg-success/10 p-3 text-sm font-medium text-success">
              <Check aria-hidden="true" className="h-4 w-4" /> {t("savedSuccess")}
            </div>
          )}

          <form onSubmit={handleSave} className="space-y-4">
            <div>
              <label htmlFor="profile-phone" className="mb-1.5 block text-xs font-bold text-muted-foreground">{t("phoneLabel")}</label>
              <input
                id="profile-phone"
                type="text"
                disabled
                value={profile?.phone || ""}
                autoComplete="tel"
                className="field-input bg-muted text-muted-foreground"
              />
            </div>

            <div>
              <label htmlFor="profile-name" className="mb-1.5 block text-xs font-bold text-muted-foreground">{t("nameLabel")}</label>
              <input
                id="profile-name"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("namePlaceholder")}
                autoComplete="name"
                disabled={loading || !profile}
                className="field-input"
              />
            </div>

            <div>
              <label htmlFor="profile-region" className="mb-1.5 block text-xs font-bold text-muted-foreground">{t("regionLabel")}</label>
              <input
                id="profile-region"
                type="text"
                value={region}
                onChange={(e) => setRegion(e.target.value)}
                placeholder={t("regionPlaceholder")}
                autoComplete="address-level1"
                disabled={loading || !profile}
                className="field-input"
              />
            </div>

            <div className="pt-2">
              <Button type="submit" variant="game" size="sm" className="w-full sm:w-auto" disabled={loading || saving || !profile}>
                {saving ? t("saving") : t("save")}
              </Button>
            </div>
          </form>
        </Card>

        <div className="max-md:hidden">
          <ChangePasswordForm />
        </div>


        <div className="max-md:hidden">
          <TelegramLinkCard />
        </div>
        <div className="max-md:hidden">
          <ReferralCard />
        </div>
        <div className="max-md:hidden">
          <PaymentHistoryCard />
        </div>

        <Card className="border-destructive/30 bg-destructive/5 p-5 max-md:hidden sm:p-6">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 className="font-display text-sm font-bold text-destructive">{t("logout")}</h3>
              <p className="text-xs text-muted-foreground">{t("logoutDescription")}</p>
            </div>
            <Button variant="outline" className="w-full border-destructive/40 text-destructive hover:bg-destructive/10 sm:w-auto" onClick={handleLogout}>
              <LogOut aria-hidden="true" className="mr-2 h-4 w-4" /> {t("logout")}
            </Button>
          </div>
        </Card>
      </div>
    </main>
  );
}
