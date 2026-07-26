export type SiteLegalDoc = {
  oferta: string;
  privacy: string;
  refund: string;
};

export type SiteLegalLocale = "uz-Latn" | "uz-Cyrl" | "ru";

export type SiteLegalBundle = {
  locales: Record<SiteLegalLocale, SiteLegalDoc>;
};

export const LEGAL_LOCALES: SiteLegalLocale[] = ["uz-Latn", "uz-Cyrl", "ru"];

export const EMPTY_LEGAL_DOC: SiteLegalDoc = {
  oferta: "",
  privacy: "",
  refund: "",
};

export function emptyLegalBundle(): SiteLegalBundle {
  return {
    locales: {
      "uz-Latn": { ...EMPTY_LEGAL_DOC },
      "uz-Cyrl": { ...EMPTY_LEGAL_DOC },
      ru: { ...EMPTY_LEGAL_DOC },
    },
  };
}

export function normalizeLegalBundle(raw: unknown): SiteLegalBundle {
  const empty = emptyLegalBundle();
  if (!raw || typeof raw !== "object") return empty;
  const locales = (raw as { locales?: unknown }).locales;
  if (!locales || typeof locales !== "object") return empty;
  const map = locales as Record<string, Partial<SiteLegalDoc>>;
  for (const loc of LEGAL_LOCALES) {
    const doc = map[loc];
    if (!doc || typeof doc !== "object") continue;
    empty.locales[loc] = {
      oferta: String(doc.oferta ?? "").trim(),
      privacy: String(doc.privacy ?? "").trim(),
      refund: String(doc.refund ?? "").trim(),
    };
  }
  return empty;
}

export type PublicSiteLegal = {
  locale: string;
  oferta: string;
  privacy: string;
  refund: string;
};

/** Prefer CMS body when non-empty; otherwise fall back to i18n sections. */
export function legalBodyOrEmpty(apiValue: string | undefined): string {
  return (apiValue ?? "").trim();
}

/**
 * Render CMS legal plain text: paragraphs split on blank lines;
 * lines starting with "## " become headings.
 */
export function parseLegalBody(body: string): { type: "h2" | "p"; text: string }[] {
  const trimmed = body.trim();
  if (!trimmed) return [];
  const blocks = trimmed.split(/\n{2,}/);
  const out: { type: "h2" | "p"; text: string }[] = [];
  for (const block of blocks) {
    const lines = block.split("\n").map((l) => l.trimEnd());
    const first = lines[0]?.trim() ?? "";
    if (first.startsWith("## ")) {
      out.push({ type: "h2", text: first.slice(3).trim() });
      const rest = lines.slice(1).join("\n").trim();
      if (rest) out.push({ type: "p", text: rest });
      continue;
    }
    out.push({ type: "p", text: lines.join("\n").trim() });
  }
  return out.filter((b) => b.text.length > 0);
}
