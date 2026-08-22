/** Namespaces that chrome (theme, locale, errors) always needs on the client. */
export const COMMON_NAMESPACES = ["ThemeToggle", "LocaleSwitcher", "Errors"] as const;

const PUBLIC_EXTRA = [
  "Landing",
  "Diagnostic",
  "Legal",
  "Narxlar",
  "Jarimalar",
  "GrandMock",
  "SchoolB2B",
  "Premium",
] as const;

const AUTH_EXTRA = ["Login", "Register", "PasswordReset", "Verify", "Profile"] as const;

const APP_EXTRA = [
  "Dashboard",
  "Sidebar",
  "Header",
  "ExamMockup",
  // The 20/50 exam chooser at /exam; KIOSK_NAMESPACES inherits it for
  // /station/exam, which renders the same component.
  "ExamPicker",
  "Tickets",
  "Signs",
  "Practice",
  "Saved",
  "Mistakes",
  "Stats",
  "Profile",
  "TelegramLink",
  "WebPush",
  "Referral",
  "PaymentHistory",
  "Premium",
  "Trial",
  "Leaderboard",
  "Arena",
  "Notifications",
  "SupportBanner",
  "MaintenanceBanner",
  "PublicSupport",
  "SupportChat",
  "SupportTicket",
  "ManualPay",
  "GrandMock",
  "SessionStart",
  "Session",
] as const;

const SESSION_EXTRA = ["Session", "GrandMock", "SessionStart", "Practice", "Saved"] as const;

const ADMIN_EXTRA = [
  "AdminNav",
  "AdminOverview",
  "AdminLogin",
  "AdminUsers",
  "AdminContent",
  "AdminReferral",
  "AdminPayments",
  "AdminPaymentsWebhooks",
  "AdminPaymentsCatalog",
  "AdminCMS",
  "AdminCMSBrand",
  "AdminCMSSurfaces",
  "AdminMonitoring",
  "AdminAnalytics",
  "AdminAnalyticsFunnels",
  "AdminAnalyticsExports",
  "AdminInvestors",
  "AdminFlags",
  "AdminB2B",
  "AdminRecon",
  "AdminAudit",
  "AdminRBAC",
  "AdminBroadcast",
  "AdminLimits",
  "AdminInbox",
  "AdminShell",
  "AdminTOTP",
  "AdminManualPay",
  "AdminTable",
  "AdminSupport",
  "OpsUsers",
  "OpsPayments",
  "OpsAudit",
  "OpsLimits",
  "OpsContacts",
  "OpsProviders",
  "OpsHealth",
] as const;

function unique(namespaces: readonly string[]): string[] {
  return [...new Set(namespaces)];
}

export const PUBLIC_NAMESPACES = unique([...COMMON_NAMESPACES, ...PUBLIC_EXTRA]);
export const AUTH_NAMESPACES = unique([...COMMON_NAMESPACES, ...AUTH_EXTRA]);
export const APP_NAMESPACES = unique([...COMMON_NAMESPACES, ...APP_EXTRA]);
export const SESSION_NAMESPACES = unique([...COMMON_NAMESPACES, ...SESSION_EXTRA]);
export const ADMIN_NAMESPACES = unique([...COMMON_NAMESPACES, ...ADMIN_EXTRA]);
/** Kiosk reuses learner tickets/practice/session screens. */
export const KIOSK_NAMESPACES = unique([...APP_NAMESPACES, ...SESSION_NAMESPACES, "Station"]);
