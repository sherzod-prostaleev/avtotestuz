"use client";

import { useEffect } from "react";
import { useLocale } from "next-intl";
import { useParams, useRouter } from "next/navigation";

/** Legacy ticket detail URL → unified chat workspace. */
export default function AdminSupportInboxRedirect() {
  const locale = useLocale();
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
  useEffect(() => {
    router.replace(`/${locale}/admin/support/inbox`);
  }, [locale, router, id]);
  return null;
}
