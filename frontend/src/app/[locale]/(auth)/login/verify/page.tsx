"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";

/** OTP verify is retired from the learner flow; keep the route as a redirect. */
export default function VerifyPage() {
  const t = useTranslations("Verify");
  const locale = useLocale();
  const router = useRouter();

  useEffect(() => {
    router.replace(`/${locale}/login`);
  }, [locale, router]);

  return (
    <p role="status" className="p-6 text-center text-sm text-muted-foreground">
      {t("redirecting")}
    </p>
  );
}
