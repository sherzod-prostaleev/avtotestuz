"use client";

import type { ReactNode } from "react";
import { useTranslations } from "next-intl";
import { Search } from "lucide-react";

export interface SignGroupPickerGroup {
  code: string;
  name: string;
}

export interface SignGroupPickerProps {
  /** The catalog's own `groups` array, so a code has exactly one label. */
  groups: SignGroupPickerGroup[];
  /**
   * Signs per group code, or `null` while the first catalog fetch is still in
   * flight — the cards then draw skeletons instead of a guessed number.
   */
  counts: Record<string, number> | null;
  /** Every sign in the catalog, or `null` for the same reason. */
  total: number | null;
  onOpenSearch: () => void;
  onPickGroup: (code: string) => void;
  className?: string;
}

/**
 * The 20px outline glyphs the approved design draws on the group cards. They
 * are the artboard's own paths rather than the nearest lucide icon: no built-in
 * matches "circle with a diagonal" or "panel with two rules", and the whole
 * point of this screen is that eight shapes are recognisable at a glance.
 */
function GroupGlyph({ children, tone }: { children: ReactNode; tone: string }) {
  return (
    <svg
      aria-hidden="true"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      className={`h-5 w-5 shrink-0 ${tone}`}
    >
      {children}
    </svg>
  );
}

/**
 * Glyph and tint per group code. Tones are written out in full because
 * Tailwind reads class names as literal source text — one assembled at runtime
 * never reaches the stylesheet.
 *
 * Three of the blues have no design token; they are the artboard's own values,
 * used once here, the same precedent as the leaderboard's bronze medal.
 */
const GROUP_STYLE: Record<string, { tone: string; glyph: ReactNode }> = {
  all: {
    tone: "text-accent",
    glyph: (
      <>
        <circle cx="12" cy="12" r="9" />
        <path d="M12 8v8" />
        <path d="M8 12h8" />
      </>
    ),
  },
  warning: {
    tone: "text-danger",
    glyph: <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3" />,
  },
  priority: {
    tone: "text-gold",
    glyph: <path d="M12 2 2 12l10 10 10-10z" />,
  },
  prohibiting: {
    tone: "text-danger",
    glyph: (
      <>
        <circle cx="12" cy="12" r="9" />
        <path d="M5.6 5.6 18.4 18.4" />
      </>
    ),
  },
  mandatory: {
    tone: "text-[hsl(210_80%_58%)]",
    glyph: (
      <>
        <circle cx="12" cy="12" r="9" />
        <path d="m8 12 3 3 5-6" />
      </>
    ),
  },
  info: {
    tone: "text-[hsl(210_60%_60%)]",
    glyph: (
      <>
        <rect x="3" y="4" width="18" height="16" rx="2" />
        <path d="M8 10h8" />
        <path d="M8 14h5" />
      </>
    ),
  },
  service: {
    tone: "text-[hsl(210_40%_66%)]",
    glyph: (
      <>
        <rect x="3" y="4" width="18" height="16" rx="2" />
        <circle cx="12" cy="12" r="3" />
      </>
    ),
  },
  supplementary: {
    tone: "text-muted-foreground",
    glyph: (
      <>
        <rect x="3" y="7" width="18" height="10" rx="2" />
        <path d="M7 12h10" />
      </>
    ),
  },
};

/**
 * The phone entry screen for /signs: a title, a search affordance and the eight
 * group cards. Picking one hands off to the catalog below it, which is the
 * layout the desktop and the classroom kiosk keep at every width — this
 * component is `md:hidden` and never renders beside it on a visible screen.
 *
 * Type sizes are the artboard's own px rather than the nearest step on the
 * scale: the app's root font is 17px, and at `text-sm` "Axborot-ko'rsatkich"
 * no longer fits a 158px card on one line.
 */
export function SignGroupPicker({
  groups,
  counts,
  total,
  onOpenSearch,
  onPickGroup,
  className = "",
}: SignGroupPickerProps) {
  const t = useTranslations("Signs");

  return (
    <div className={`flex flex-col gap-3 ${className}`}>
      <div>
        <h1 className="font-display text-2xl font-bold leading-[1.15] tracking-tight">
          {t("entryHeading")}
        </h1>
        <p className="text-[15px] leading-[21px] text-muted-foreground">
          {total === null ? (
            <span
              aria-hidden="true"
              className="mt-1.5 block h-3 w-44 animate-pulse rounded bg-border"
            />
          ) : (
            t("entrySummary", { total, groups: groups.length - 1 })
          )}
        </p>
      </div>

      {/* Reveals the catalog's real <input type="search"> rather than being a
          second search field of its own. */}
      <button
        type="button"
        onClick={onOpenSearch}
        className="flex h-[46px] items-center gap-2.5 rounded-xl border border-border bg-card px-3 text-left"
      >
        <Search aria-hidden="true" className="h-[18px] w-[18px] shrink-0 text-muted-foreground/80" />
        <span className="text-[15px] text-muted-foreground/75">{t("entrySearchPlaceholder")}</span>
      </button>

      <div className="grid grid-cols-2 gap-2">
        {groups.map((group) => {
          const style = GROUP_STYLE[group.code];
          const count =
            counts === null || total === null
              ? null
              : group.code === "all"
                ? total
                : counts[group.code] ?? 0;

          return (
            <button
              key={group.code}
              type="button"
              onClick={() => onPickGroup(group.code)}
              className="flex min-h-[74px] flex-col justify-between gap-2 rounded-xl border border-border bg-card px-[10px] py-[9px] text-left"
            >
              {style ? <GroupGlyph tone={style.tone}>{style.glyph}</GroupGlyph> : null}
              <span>
                <span className="block text-[15px] font-bold leading-[19px]">{group.name}</span>
                <span className="mt-px block text-[13px] leading-[17px] text-muted-foreground">
                  {count === null ? (
                    <span
                      aria-hidden="true"
                      className="mt-0.5 block h-2.5 w-16 animate-pulse rounded bg-border"
                    />
                  ) : (
                    t("groupSignCount", { count })
                  )}
                </span>
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
