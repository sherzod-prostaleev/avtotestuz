/** Nav item → required permission (any-of). Empty = always visible when authenticated. */
export const ADMIN_ROUTE_PERMISSIONS: Record<string, string[]> = {
  monitoring: ["monitoring.read"],
  analytics: ["analytics.read"],
  investors: ["investors.read"],
  users: ["users.read"],
  content: ["content.questions.read"],
  payments: ["payments.read", "referral.read"],
  cms: ["cms.read"],
  settings: ["settings.flags", "settings.config"],
  security: ["security.audit.read", "security.rbac"],
  support: ["support.inbox", "support.broadcast"],
  b2b: ["users.read"],
};

export function hasPermission(
  permissions: string[] | undefined | null,
  required: string | string[],
): boolean {
  if (!permissions?.length) return false;
  const need = Array.isArray(required) ? required : [required];
  if (need.length === 0) return true;
  const set = new Set(permissions);
  return need.some((p) => set.has(p));
}

export function hasAllPermissions(
  permissions: string[] | undefined | null,
  required: string[],
): boolean {
  if (!permissions?.length) return false;
  const set = new Set(permissions);
  return required.every((p) => set.has(p));
}

/** Map href suffix after /admin to permission group key. */
export function routePermissionKey(href: string, locale: string): string | null {
  const base = `/${locale}/admin`;
  if (href === base || href === `${base}/`) return null;
  const rest = href.slice(base.length).replace(/^\//, "");
  const top = rest.split("/")[0];
  return top || null;
}
