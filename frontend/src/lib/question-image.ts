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
