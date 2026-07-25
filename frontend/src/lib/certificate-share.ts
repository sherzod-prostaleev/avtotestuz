/** Build absolute public certificate URL for a share code. */
export function certificateShareUrl(
  origin: string,
  locale: string,
  shareCode: string,
): string {
  const base = origin.replace(/\/$/, "");
  return `${base}/${locale}/sertifikat/${encodeURIComponent(shareCode)}`;
}

/** Prefer Web Share; fall back to clipboard. Returns which path succeeded. */
export async function shareOrCopyCertificateLink(opts: {
  url: string;
  title: string;
  text: string;
}): Promise<"share" | "clipboard"> {
  if (typeof navigator !== "undefined" && typeof navigator.share === "function") {
    try {
      await navigator.share({ title: opts.title, text: opts.text, url: opts.url });
      return "share";
    } catch (err) {
      // User cancel should not fall through to clipboard spam.
      if (err instanceof DOMException && err.name === "AbortError") {
        throw err;
      }
    }
  }
  await navigator.clipboard.writeText(opts.url);
  return "clipboard";
}
