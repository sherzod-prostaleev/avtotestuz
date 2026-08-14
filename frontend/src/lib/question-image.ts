/**
 * Local public asset shown when an official exam / session question has no
 * media URL. Served by Next from `frontend/public/exam/`.
 */
export const QUESTION_IMAGE_PLACEHOLDER = "/exam/placeholder-driver-go-cars.png";

/**
 * API MEDIA_BASE_URL is http://localhost:9000/media in local .env. That origin is
 * not `'self'` on :3000, and CSP `img-src` only allows `'self' data: blob: https:`,
 * so the browser blocks the real diagram even when MinIO returns 200.
 * Production nginx already serves the bucket at same-origin `/media/...`.
 */
const LOCAL_MINIO_MEDIA =
  /^https?:\/\/(?:localhost|127\.0\.0\.1|minio):9000\/media(\/.*)?$/i;

/** Map a local MinIO absolute URL onto same-origin `/media/...`. Other URLs pass through. */
export function toBrowserMediaUrl(imageUrl: string): string {
  const trimmed = imageUrl.trim();
  const match = LOCAL_MINIO_MEDIA.exec(trimmed);
  if (!match) return trimmed;
  return `/media${match[1] ?? ""}`;
}

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
  return trimmed ? toBrowserMediaUrl(trimmed) : QUESTION_IMAGE_PLACEHOLDER;
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
    if (raw) urls.push(toBrowserMediaUrl(raw));
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
    if (!trimmed) continue;
    const src = toBrowserMediaUrl(trimmed);
    if (src === QUESTION_IMAGE_PLACEHOLDER || seen.has(src)) continue;
    seen.add(src);
    const img = new Image();
    img.src = src;
  }
}
