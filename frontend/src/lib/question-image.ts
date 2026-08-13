/**
 * Local public asset shown when an official exam / session question has no
 * media URL. Served by Next from `frontend/public/exam/`.
 */
export const QUESTION_IMAGE_PLACEHOLDER = "/exam/placeholder-driver-go-cars.png";

/** True when the API provided a non-empty media URL (do not overwrite real diagrams). */
export function hasQuestionImage(imageUrl?: string | null): boolean {
  return Boolean(imageUrl?.trim());
}

/**
 * Resolve the URL to render for a question media slot.
 * Real image URLs win; null/empty/whitespace fall back to the Driver Go cars placeholder.
 */
export function resolveQuestionImageUrl(imageUrl?: string | null): string {
  const trimmed = imageUrl?.trim();
  return trimmed || QUESTION_IMAGE_PLACEHOLDER;
}

/** Current question plus the next `ahead` items that actually have media. */
export function upcomingQuestionImageUrls(
  questions: ReadonlyArray<{ image_url?: string | null }>,
  currentIndex: number,
  ahead = 2
): string[] {
  if (!Array.isArray(questions) || currentIndex < 0) return [];
  const last = Math.min(questions.length - 1, currentIndex + ahead);
  const urls: string[] = [];
  for (let i = currentIndex; i <= last; i += 1) {
    const raw = questions[i]?.image_url?.trim();
    if (raw) urls.push(raw);
  }
  return urls;
}

/**
 * Warm the browser cache for upcoming question diagrams. Skips the shared
 * placeholder so a 200-question text session does not hammer one PNG.
 * Best-effort: no-op on the server and never throws.
 */
export function prefetchQuestionImages(urls: Array<string | null | undefined>): void {
  if (typeof window === "undefined") return;
  const seen = new Set<string>();
  for (const url of urls) {
    const trimmed = url?.trim();
    if (!trimmed || trimmed === QUESTION_IMAGE_PLACEHOLDER || seen.has(trimmed)) continue;
    seen.add(trimmed);
    const img = new Image();
    img.src = trimmed;
  }
}
