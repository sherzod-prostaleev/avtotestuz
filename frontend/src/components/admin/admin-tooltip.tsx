"use client";

import * as Tooltip from "@radix-ui/react-tooltip";
import { Info } from "lucide-react";

export type AdminTooltipContent = {
  /** Nima qiladi */
  what: string;
  /** Nima uchun */
  why?: string;
  /** Qachon ishlatish */
  when?: string;
  /** Xavflar */
  risks?: string;
  /** Tavsiya */
  recommend?: string;
};

type AdminTooltipProps = {
  content: AdminTooltipContent;
  label?: string;
};

/** ⓘ advanced-setting helper — hover/focus/click. */
export function AdminTooltip({ content, label = "Ma'lumot" }: AdminTooltipProps) {
  return (
    <Tooltip.Provider delayDuration={200}>
      <Tooltip.Root>
        <Tooltip.Trigger asChild>
          <button
            type="button"
            className="inline-flex h-6 w-6 items-center justify-center rounded-md text-muted-foreground transition hover:bg-white/[0.06] hover:text-accent focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            aria-label={label}
          >
            <Info className="h-3.5 w-3.5" aria-hidden />
          </button>
        </Tooltip.Trigger>
        <Tooltip.Portal>
          <Tooltip.Content
            side="top"
            sideOffset={6}
            className="z-50 max-w-xs rounded-xl border border-border/80 bg-[hsl(220_28%_10%)] px-3 py-2.5 text-xs leading-relaxed text-foreground shadow-xl"
          >
            <p className="font-semibold text-foreground">{content.what}</p>
            {content.why ? (
              <p className="mt-1.5 text-muted-foreground">
                <span className="font-semibold text-foreground/80">Nima uchun: </span>
                {content.why}
              </p>
            ) : null}
            {content.when ? (
              <p className="mt-1 text-muted-foreground">
                <span className="font-semibold text-foreground/80">Qachon: </span>
                {content.when}
              </p>
            ) : null}
            {content.risks ? (
              <p className="mt-1 text-amber-200/90">
                <span className="font-semibold">Xavf: </span>
                {content.risks}
              </p>
            ) : null}
            {content.recommend ? (
              <p className="mt-1 text-emerald-300/90">
                <span className="font-semibold">Tavsiya: </span>
                {content.recommend}
              </p>
            ) : null}
            <Tooltip.Arrow className="fill-[hsl(220_28%_10%)]" />
          </Tooltip.Content>
        </Tooltip.Portal>
      </Tooltip.Root>
    </Tooltip.Provider>
  );
}
