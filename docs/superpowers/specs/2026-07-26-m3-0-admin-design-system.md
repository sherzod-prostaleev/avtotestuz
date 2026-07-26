# M3-0 — Admin Design System v1

Locked for Admin Control Center. Do not invent per-page spacing/type.

## Tokens (Asphalt & Signal)

- Surface: `hsl(220 28% 6–9%)` shell / card
- Accent: CSS `--accent` (amber signal)
- Radius: controls `rounded-xl`, panels `rounded-2xl`, hero shell `rounded-3xl`
- Type: `font-display` for titles; body `text-sm`; labels `text-[11px] uppercase tracking-wider`
- Focus: `focus-visible:ring-2 focus-visible:ring-ring`

## Primitives (`frontend/src/components/admin/`)

| Component | Use |
|-----------|-----|
| `AdminPageHeader` | Title + badge + actions |
| `MetricTile` | KPI (+ optional sparkline) |
| `AdminDataTable` | Virtualized directories |
| `AdminTooltip` | ⓘ advanced settings |
| `DangerConfirm` | Typed confirm for destructive ops |
| `PermissionGate` | FE mirror of RBAC (API still authoritative) |
| `SecretField` | Masked secret + reveal-once |
| `AuditDiff` | before/after JSON |
| `ComingSoon` | Honest stubs — no fake data |
| `AdminCommandPalette` | ⌘K route jump |

## TOTP policy

- Enroll/confirm/disable via `/admin/v1/security/totp/*`
- Login step-up when `totp_secret_enc` set
- Enforce enroll banner for superadmin when `ADMIN_TOTP_ENFORCE=1|true`
