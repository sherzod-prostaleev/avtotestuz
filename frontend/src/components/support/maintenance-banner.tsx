"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { apiGet } from "@/lib/api-client";

type FlagsDTO = {
  maintenance_mode?: boolean;
};

/** Learner-facing strip when feature_flag maintenance_mode is true. */
export function MaintenanceBanner() {
  const t = useTranslations("MaintenanceBanner");
  const [on, setOn] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void apiGet<FlagsDTO>("flags")
      .then((f) => {
        if (!cancelled) setOn(Boolean(f.maintenance_mode));
      })
      .catch(() => {
        /* optional chrome */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (!on) return null;

  return (
    <div
      role="status"
      className="border-b border-border bg-destructive/10 px-4 py-2 text-center text-sm font-semibold text-foreground"
    >
      {t("message")}
    </div>
  );
}
