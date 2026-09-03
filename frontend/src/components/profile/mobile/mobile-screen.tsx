"use client";

import { useTranslations } from "next-intl";
import { ChevronLeft } from "lucide-react";

type Props = {
  title: string;
  onBack: () => void;
  /**
   * Pinned to the bottom edge by `mt-auto`, which is how the artboard's
   * `flex: 1` spacer above the CTA is drawn. Screens without a CTA pass nothing
   * and simply stack from the top.
   */
  footer?: React.ReactNode;
  /** Artboard gap between blocks: 12px on most screens, 11px on the two money ones. */
  gapClassName?: string;
  children: React.ReactNode;
};

/**
 * The chrome every profile sub-screen shares: a back chevron, a title, and the
 * exact height a phone has left over (`profile-screen-fit`, which collapses to
 * `min-height: 0` from 768px up). Sub-screens are panels rather than routes, so
 * this lives entirely inside the page's `md:hidden` subtree and the wide layout
 * never sees it.
 */
export function MobileScreen({ title, onBack, footer, gapClassName = "gap-3", children }: Props) {
  const t = useTranslations("Profile");

  return (
    <section className={`profile-screen-fit flex flex-col ${gapClassName}`}>
      <div className="flex items-center gap-0.5">
        {/* 44px tap target, pulled left by its own padding so the chevron's edge
            still lines up with the content column the artboard draws. */}
        <button
          type="button"
          onClick={onBack}
          aria-label={t("back")}
          className="-ml-3 flex h-11 w-11 shrink-0 items-center justify-center rounded-xl text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <ChevronLeft aria-hidden="true" className="h-[22px] w-[22px]" />
        </button>
        <h2 className="min-w-0 flex-1 truncate font-display text-xl font-extrabold tracking-tight">
          {title}
        </h2>
      </div>

      {children}

      {footer ? <div className="mt-auto pb-3">{footer}</div> : null}
    </section>
  );
}
