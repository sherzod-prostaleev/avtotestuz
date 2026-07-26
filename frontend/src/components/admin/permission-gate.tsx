"use client";

import { useTranslations } from "next-intl";
import { useAdminMeOptional } from "@/components/admin/admin-me-context";
import { hasAllPermissions, hasPermission } from "@/lib/admin-permissions";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";

type PermissionGateProps = {
  permission: string | string[];
  /** If true, every listed permission is required. Default: any-of. */
  requireAll?: boolean;
  children: React.ReactNode;
  fallback?: React.ReactNode;
  mode?: "hide" | "deny";
};

export function PermissionGate({
  permission,
  requireAll = false,
  children,
  fallback,
  mode = "deny",
}: PermissionGateProps) {
  const t = useTranslations("AdminShell");
  const me = useAdminMeOptional();
  const perms = me?.permissions;
  const ok = requireAll
    ? hasAllPermissions(perms, Array.isArray(permission) ? permission : [permission])
    : hasPermission(perms, permission);

  if (ok) return <>{children}</>;
  if (mode === "hide") return null;
  if (fallback) return <>{fallback}</>;
  return (
    <AdminEmptyState
      title={t("permissionDeniedTitle")}
      description={t("permissionDeniedBody")}
    />
  );
}
