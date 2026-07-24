"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter, usePathname } from "next/navigation";
import { apiGet, apiPatch } from "@/lib/api-client";
import { ThemeToggle } from "@/components/theme-toggle";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft, User, Globe, LogOut, Check } from "lucide-react";
import { ReferralCard } from "@/components/profile/referral-card";
import { PaymentHistoryCard } from "@/components/profile/payment-history-card";

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
  const pathname = usePathname();

  const [profile, setProfile] = useState<UserProfileData | null>(null);
  const [name, setName] = useState<string>("");
  const [region, setRegion] = useState<string>("");
  const [loading, setLoading] = useState<boolean>(true);
  const [saving, setSaving] = useState<boolean>(false);
  const [savedSuccess, setSavedSuccess] = useState<boolean>(false);
  const [errorKey, setErrorKey] = useState<"loadError" | "saveError" | null>(null);

  const loadProfile = useCallback(async () => {
    setLoading(true);
    setErrorKey(null);
    try {
      const data = await apiGet<MeResponse>("me");
      setProfile(data.profile);
      setName(data.profile.name);
      setRegion(data.profile.region);
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

  const handleLanguageChange = async (newLocale: string) => {
    if (newLocale === currentLocale) return;
    const newPath = pathname.replace(`/${currentLocale}`, `/${newLocale}`);
    try {
      await apiPatch<UserProfileData>("me", { locale_pref: newLocale });
    } finally {
      router.push(newPath);
    }
  };

  const handleLogout = async () => {
    try {
      await fetch("/api/auth/logout", { method: "POST" });
    } finally {
      router.push(`/${currentLocale}/login`);
    }
  };

  return (
    <main className="mx-auto max-w-4xl px-4 py-8">
      <header className="mb-6">
        <Link href={`/${currentLocale}/dashboard`} className="mb-2 flex items-center gap-1 text-sm text-accent hover:underline">
          <ArrowLeft aria-hidden="true" className="h-4 w-4" /> {t("backHome")}
        </Link>
        <h1 className="font-display text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </header>

      <div className="space-y-6">
        {/* User Info Form */}
        <Card className="p-6">
          <CardHeader className="p-0 mb-4 flex flex-row items-center gap-2">
            <User aria-hidden="true" className="h-5 w-5 text-accent" />
            <CardTitle className="text-base font-bold">{t("personalInfo")}</CardTitle>
          </CardHeader>

          {loading && (
            <div role="status" className="mb-4 text-sm text-muted-foreground">
              {t("loading")}
            </div>
          )}

          {errorKey && (
            <div role="alert" className="mb-4 rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
              <p>{t(errorKey)}</p>
              {errorKey === "loadError" && (
                <Button type="button" variant="outline" size="sm" className="mt-3" onClick={() => void loadProfile()}>
                  {t("retry")}
                </Button>
              )}
            </div>
          )}

          {savedSuccess && (
            <div role="status" className="mb-4 flex items-center gap-2 rounded-md border border-success/50 bg-success/10 p-3 text-sm text-success font-medium">
              <Check aria-hidden="true" className="h-4 w-4" /> {t("savedSuccess")}
            </div>
          )}

          <form onSubmit={handleSave} className="space-y-4">
            <div>
              <label htmlFor="profile-phone" className="mb-1 block text-xs font-bold text-muted-foreground">{t("phoneLabel")}</label>
              <input
                id="profile-phone"
                type="text"
                disabled
                value={profile?.phone || ""}
                autoComplete="tel"
                className="w-full rounded-md border border-border bg-background/50 p-2.5 text-sm text-muted-foreground"
              />
            </div>

            <div>
              <label htmlFor="profile-name" className="mb-1 block text-xs font-bold text-muted-foreground">{t("nameLabel")}</label>
              <input
                id="profile-name"
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t("namePlaceholder")}
                autoComplete="name"
                disabled={loading || !profile}
                className="w-full rounded-md border border-border bg-card p-2.5 text-sm text-foreground focus:border-accent focus:outline-none"
              />
            </div>

            <div>
              <label htmlFor="profile-region" className="mb-1 block text-xs font-bold text-muted-foreground">{t("regionLabel")}</label>
              <input
                id="profile-region"
                type="text"
                value={region}
                onChange={(e) => setRegion(e.target.value)}
                placeholder={t("regionPlaceholder")}
                autoComplete="address-level1"
                disabled={loading || !profile}
                className="w-full rounded-md border border-border bg-card p-2.5 text-sm text-foreground focus:border-accent focus:outline-none"
              />
            </div>

            <Button type="submit" variant="game" size="sm" disabled={loading || saving || !profile}>
              {saving ? t("saving") : t("save")}
            </Button>
          </form>
        </Card>

        {/* Language & Theme Settings */}
        <Card className="p-6">
          <CardHeader className="p-0 mb-4 flex flex-row items-center gap-2">
            <Globe aria-hidden="true" className="h-5 w-5 text-accent" />
            <CardTitle className="text-base font-bold">{t("appearanceSettings")}</CardTitle>
          </CardHeader>

          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-bold">{t("language")}</p>
                <p className="text-xs text-muted-foreground">{t("languageDescription")}</p>
              </div>
              <div role="group" aria-label={t("language")} className="flex gap-1 rounded-md border border-border bg-card p-1">
                {[
                  { code: "uz-Latn", label: t("languageUzLatn") },
                  { code: "uz-Cyrl", label: t("languageUzCyrl") },
                  { code: "ru", label: t("languageRu") },
                ].map((lang) => (
                  <button
                    type="button"
                    key={lang.code}
                    onClick={() => void handleLanguageChange(lang.code)}
                    aria-pressed={currentLocale === lang.code}
                    className={`rounded px-3 py-1 text-xs font-bold transition-all ${
                      currentLocale === lang.code
                        ? "bg-accent text-accent-foreground"
                        : "text-muted-foreground hover:text-foreground"
                    }`}
                  >
                    {lang.label}
                  </button>
                ))}
              </div>
            </div>

            <div className="flex items-center justify-between border-t border-border pt-4">
              <div>
                <p className="text-sm font-bold">{t("theme")}</p>
                <p className="text-xs text-muted-foreground">{t("themeDescription")}</p>
              </div>
              <ThemeToggle />
            </div>
          </div>
        </Card>

        {/* Referral Program Section */}
        <ReferralCard />

        {/* Payment History Section */}
        <PaymentHistoryCard />

        {/* Logout Section */}
        <Card className="p-6 border-destructive/30 bg-destructive/5">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="font-display text-sm font-bold text-destructive">{t("logout")}</h3>
              <p className="text-xs text-muted-foreground">{t("logoutDescription")}</p>
            </div>
            <Button variant="outline" className="border-destructive/40 text-destructive hover:bg-destructive/10" onClick={handleLogout}>
              <LogOut aria-hidden="true" className="mr-2 h-4 w-4" /> {t("logout")}
            </Button>
          </div>
        </Card>
      </div>
    </main>
  );
}
