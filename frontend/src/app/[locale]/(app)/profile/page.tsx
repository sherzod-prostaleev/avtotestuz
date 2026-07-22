"use client";

import { useState, useEffect } from "react";
import { useTranslations, useLocale } from "next-intl";
import Link from "next/link";
import { useRouter, usePathname } from "next/navigation";
import { apiGet, apiPatch } from "@/lib/api-client";
import { ThemeToggle } from "@/components/theme-toggle";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { ArrowLeft, User, Globe, Moon, LogOut, Check } from "lucide-react";

interface UserProfileData {
  id: string;
  phone: string;
  name?: string;
  region?: string;
}

export default function ProfilePage() {
  const t = useTranslations("Profile");
  const currentLocale = useLocale();
  const router = useRouter();
  const pathname = usePathname();

  const [profile, setProfile] = useState<UserProfileData | null>(null);
  const [name, setName] = useState<string>("");
  const [region, setRegion] = useState<string>("");
  const [saving, setSaving] = useState<boolean>(false);
  const [savedSuccess, setSavedSuccess] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function loadProfile() {
      try {
        const data = await apiGet<UserProfileData>("me");
        setProfile(data);
        setName(data?.name ?? "");
        setRegion(data?.region ?? "");
      } catch {
        setError("Profil ma'lumotlarini yuklab bo'lmadi");
      }
    }
    loadProfile();
  }, []);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    setSavedSuccess(false);
    try {
      const updated = await apiPatch<UserProfileData>("me", { name, region });
      setProfile(updated);
      setSavedSuccess(true);
      setTimeout(() => setSavedSuccess(false), 3000);
    } catch {
      setError("Saqlashda xatolik yuz berdi");
    } finally {
      setSaving(false);
    }
  };

  const handleLanguageChange = (newLocale: string) => {
    if (newLocale === currentLocale) return;
    const newPath = pathname.replace(`/${currentLocale}`, `/${newLocale}`);
    router.push(newPath);
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
          <ArrowLeft className="h-4 w-4" /> Bosh sahifaga qaytish
        </Link>
        <h1 className="font-display text-2xl font-bold">{t("title")}</h1>
        <p className="text-sm text-muted-foreground">{t("subtitle")}</p>
      </header>

      <div className="space-y-6">
        {/* User Info Form */}
        <Card className="p-6">
          <CardHeader className="p-0 mb-4 flex flex-row items-center gap-2">
            <User className="h-5 w-5 text-accent" />
            <CardTitle className="text-base font-bold">Shaxsiy ma'lumotlar</CardTitle>
          </CardHeader>

          {error && (
            <div className="mb-4 rounded-md border border-destructive/50 bg-destructive/10 p-3 text-sm text-destructive">
              {error}
            </div>
          )}

          {savedSuccess && (
            <div className="mb-4 flex items-center gap-2 rounded-md border border-success/50 bg-success/10 p-3 text-sm text-success font-medium">
              <Check className="h-4 w-4" /> {t("savedSuccess")}
            </div>
          )}

          <form onSubmit={handleSave} className="space-y-4">
            <div>
              <label className="mb-1 block text-xs font-bold text-muted-foreground">Telefon raqam</label>
              <input
                type="text"
                disabled
                value={profile?.phone || ""}
                className="w-full rounded-md border border-border bg-background/50 p-2.5 text-sm text-muted-foreground"
              />
            </div>

            <div>
              <label className="mb-1 block text-xs font-bold text-muted-foreground">{t("nameLabel")}</label>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Ismingizni kiriting"
                className="w-full rounded-md border border-border bg-card p-2.5 text-sm text-foreground focus:border-accent focus:outline-none"
              />
            </div>

            <div>
              <label className="mb-1 block text-xs font-bold text-muted-foreground">{t("regionLabel")}</label>
              <input
                type="text"
                value={region}
                onChange={(e) => setRegion(e.target.value)}
                placeholder="Masalan: Toshkent shahri"
                className="w-full rounded-md border border-border bg-card p-2.5 text-sm text-foreground focus:border-accent focus:outline-none"
              />
            </div>

            <Button type="submit" variant="game" size="sm" disabled={saving}>
              {saving ? "Saqlanmoqda..." : t("save")}
            </Button>
          </form>
        </Card>

        {/* Language & Theme Settings */}
        <Card className="p-6">
          <CardHeader className="p-0 mb-4 flex flex-row items-center gap-2">
            <Globe className="h-5 w-5 text-accent" />
            <CardTitle className="text-base font-bold">Til va Mavzu sozlamalari</CardTitle>
          </CardHeader>

          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm font-bold">{t("language")}</p>
                <p className="text-xs text-muted-foreground">O'zbek tili (Lotin/Kirill) yoki Rus tili</p>
              </div>
              <div className="flex gap-1 rounded-md border border-border bg-card p-1">
                {[
                  { code: "uz-Latn", label: "O'zb" },
                  { code: "uz-Cyrl", label: "Ўзб" },
                  { code: "ru", label: "Рус" },
                ].map((lang) => (
                  <button
                    key={lang.code}
                    onClick={() => handleLanguageChange(lang.code)}
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
                <p className="text-xs text-muted-foreground">Qorong'i (Dark) yoki Yorug' (Light) tema</p>
              </div>
              <ThemeToggle />
            </div>
          </div>
        </Card>

        {/* Logout Section */}
        <Card className="p-6 border-destructive/30 bg-destructive/5">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="font-display text-sm font-bold text-destructive">{t("logout")}</h3>
              <p className="text-xs text-muted-foreground">Akkauntdan chiqish va sessiyani yakunlash</p>
            </div>
            <Button variant="outline" className="border-destructive/40 text-destructive hover:bg-destructive/10" onClick={handleLogout}>
              <LogOut className="mr-2 h-4 w-4" /> {t("logout")}
            </Button>
          </div>
        </Card>
      </div>
    </main>
  );
}
