/**
 * Deliberately hostile row fixtures for the 390px admin gate.
 *
 * An empty table cannot overflow, so a responsive gate fed empty responses
 * proves almost nothing about the thing it exists to catch. These rows are
 * shaped like production data but pushed to the wide end of plausible: full
 * Uzbek question text rather than a stub, real 16-digit card numbers, long
 * addresses, eight-figure sums, and unbroken tokens with no spaces for the
 * layout to wrap on.
 *
 * If a column can force a phone to scroll sideways, it does it here.
 */

const LONG_QUESTION =
  "Svetofor va yo‘l harakati boshqaruvchisining ishoralari bir vaqtning o‘zida " +
  "berilgan bo‘lsa, haydovchi qaysi biriga bo‘ysunishi shart va bu holatda " +
  "chorrahadan o‘tish tartibi qanday belgilanadi?";

const LONG_SUBJECT =
  "To‘lov amalga oshdi, lekin premium ochilmadi — Payme orqali to‘ladim, pul yechildi";

/** No spaces: the worst case for any wrapping strategy. */
const UNBREAKABLE = "PAYME-TXN-6a3f9c21b8d74e5fa0c1927d3e8b4f60-RETRY-000042";

export const ADMIN_ROW_FIXTURES: Record<string, unknown> = {
  "/api/admin/content/questions": {
    items: Array.from({ length: 12 }, (_, i) => ({
      id: `q-${i}-6a3f9c21-b8d7-4e5f-a0c1-927d3e8b4f60`,
      source_ext_id: `AVTOTEST-2026-BILET-${String(i + 1).padStart(4, "0")}`,
      category_code: "yol_belgilari_va_chiziqlari",
      category_name: "Yo‘l belgilari va chiziqlari",
      validation_status: "needs_review",
      text_preview: LONG_QUESTION,
      variant_numbers: [1, 2, 3, 4, 5, 6],
      explanation_status: "verified",
      updated_at: "2026-07-28T14:03:11Z",
    })),
    page: 1,
    limit: 20,
    total: 1260,
  },

  "/api/admin/content/tickets": {
    items: Array.from({ length: 12 }, (_, i) => ({
      number: i + 1,
      question_count: 20,
      sort_order: (i + 1) * 10,
    })),
    page: 1,
    limit: 20,
    total: 63,
  },

  "/api/admin/content/signs": {
    items: Array.from({ length: 12 }, (_, i) => ({
      code: `5.${i + 1}.3`,
      group_code: "axborot_ishora_belgilari",
      name: "Yo‘lning boshlanishi va tugashini bildiruvchi axborot-ishora belgisi",
      question_count: 14,
      has_image: true,
    })),
    page: 1,
    limit: 20,
    total: 285,
  },

  "/api/admin/content/explanations": {
    items: Array.from({ length: 12 }, (_, i) => ({
      question_id: `q-${i}-6a3f9c21-b8d7-4e5f-a0c1-927d3e8b4f60`,
      source_ext_id: `AVTOTEST-2026-BILET-${String(i + 1).padStart(4, "0")}`,
      text_preview: LONG_QUESTION,
      locale: "uz-Latn",
      status: "pending",
      source: "ai_generated_reviewed_by_operator",
      category_code: "yol_belgilari_va_chiziqlari",
      explanation_id: `e-${i}-11112222-3333-4444-5555-666677778888`,
      verified_at: undefined,
    })),
    page: 1,
    limit: 20,
    total: 940,
  },

  "/api/admin/payments/transactions": {
    items: Array.from({ length: 12 }, (_, i) => ({
      id: `p-${i}-aaaabbbb-cccc-dddd-eeee-ffff00001111`,
      profile_id: `u-${i}-99998888-7777-6666-5555-444433332222`,
      phone_masked: "+998 9* *** ** 67",
      provider: i % 2 === 0 ? "payme" : "click",
      status: i % 3 === 0 ? "paid" : "pending",
      amount_uzs: 149000 + i * 10000,
      tariff_code: UNBREAKABLE,
      created_at: "2026-07-28T14:03:11Z",
      paid_at: "2026-07-28T14:04:02Z",
    })),
    page: 1,
    limit: 20,
    total: 8421,
  },

  "/api/admin/referral/payouts": {
    items: Array.from({ length: 12 }, (_, i) => ({
      id: `r-${i}-aaaabbbb-cccc-dddd-eeee-ffff00001111`,
      profile_id: `u-${i}-99998888-7777-6666-5555-444433332222`,
      profile_phone: "+998901234567",
      profile_name: "Musharrafxon Abdurahmonova-Toshpo‘latova",
      amount_uzs: 1_250_000 + i * 25_000,
      card_masked: "8600 **** **** 9012",
      card_network: "uzcard",
      status: i % 2 === 0 ? "pending" : "paid",
      admin_note: "",
      created_at: "2026-07-28T14:03:11Z",
    })),
    page: 1,
    limit: 20,
    total: 137,
  },

  "/api/admin/support/conversations": {
    items: Array.from({ length: 12 }, (_, i) => ({
      id: `s-${i}-aaaabbbb-cccc-dddd-eeee-ffff00001111`,
      profile_id: `u-${i}-99998888-7777-6666-5555-444433332222`,
      profile_name: "Musharrafxon Abdurahmonova",
      profile_phone: "+998901234567",
      status: "open",
      unread_admin: i % 3,
      unread_user: 0,
      preview: LONG_SUBJECT,
      last_message_at: "2026-07-28T15:22:40Z",
      created_at: "2026-07-28T14:03:11Z",
      updated_at: "2026-07-28T15:22:40Z",
    })),
    page: 1,
    limit: 20,
    total: 302,
  },

  // The manual-payment page reads `data` as arrays directly rather than the
  // paginated `{items: []}` envelope used by admin directories.
  "/api/admin/payments/manual/cards": [],
  "/api/admin/payments/manual/queue": [],
  "/api/admin/payments/manual/events": [],
  "/api/admin/payments/manual/telegram": {
    configured: false,
    has_api_id: false,
    has_api_hash: false,
    has_session: false,
    humo_bot_username: "HUMOcardbot",
  },

  "/api/admin/users": {
    items: Array.from({ length: 12 }, (_, i) => ({
      id: `u-${i}-99998888-7777-6666-5555-444433332222`,
      phone: "+998901234567",
      phone_masked: "+998 9* *** ** 67",
      name: "Musharrafxon Abdurahmonova-Toshpo‘latova",
      locale_pref: "uz-Latn",
      status: "active",
      vip_active: true,
      has_password: true,
      streak: 128,
      created_at: "2026-07-28T14:03:11Z",
      last_seen_at: "2026-07-29T09:11:05Z",
      referral_code: "MUSHARRAF2026",
    })),
    page: 1,
    limit: 20,
    total: 5140,
  },
};

/**
 * The RBAC matrix keeps a real <table> and is the one admin surface with no
 * card projection, so it is the most likely to overflow — and it was being
 * measured empty. Production shape: 7 roles x 30 permissions.
 */
const RBAC_ROLES = [
  "superadmin", "admin", "finance", "content", "support", "analyst", "auditor",
];
const RBAC_PERMS = [
  "monitoring.read", "analytics.read", "investors.read", "users.read",
  "users.block", "users.sessions.revoke", "users.entitlements.grant",
  "content.questions.read", "content.questions.write", "content.signs.write",
  "payments.read", "payments.void", "payments.refund", "referral.read",
  "referral.payouts.manage", "referral.rates.manage", "cms.read", "cms.write",
  "settings.flags", "settings.config", "settings.limits", "security.audit.read",
  "security.rbac", "security.ip", "support.inbox", "support.broadcast",
  "b2b.read", "b2b.write", "exports.run", "jobs.manage",
];

ADMIN_ROW_FIXTURES["/api/admin/security/rbac"] = {
  roles: RBAC_ROLES.map((code, i) => ({
    code,
    name: `${code} roli`,
    description: "Ushbu rol uchun to‘liq tavsif matni",
    permissions: RBAC_PERMS.filter((_, j) => (i + j) % (i + 2) !== 0),
  })),
  permissions: RBAC_PERMS.map((code) => ({
    code,
    description: "Bu ruxsat nimaga imkon berishini tushuntiruvchi uzun tavsif",
  })),
};
