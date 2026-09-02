/**
 * Standard date formatter for Avtotest.
 * Formats dates cleanly as DD.MM.YY (e.g. 24.07.26) or DD.MM.YY HH:mm (e.g. 24.07.26 22:38).
 */

export function formatDateShort(value: string | Date | null | undefined): string {
  if (!value) return "";
  const date = typeof value === "string" ? new Date(value) : value;
  if (Number.isNaN(date.getTime())) return typeof value === "string" ? value : "";

  const d = date.getDate().toString().padStart(2, "0");
  const m = (date.getMonth() + 1).toString().padStart(2, "0");
  const y = date.getFullYear().toString().slice(-2);

  return `${d}.${m}.${y}`;
}

export function formatDateWithTime(value: string | Date | null | undefined): string {
  if (!value) return "";
  const date = typeof value === "string" ? new Date(value) : value;
  if (Number.isNaN(date.getTime())) return typeof value === "string" ? value : "";

  const d = date.getDate().toString().padStart(2, "0");
  const m = (date.getMonth() + 1).toString().padStart(2, "0");
  const y = date.getFullYear().toString().slice(-2);

  const hh = date.getHours().toString().padStart(2, "0");
  const mm = date.getMinutes().toString().padStart(2, "0");

  return `${d}.${m}.${y} ${hh}:${mm}`;
}

/**
 * Calendar days between `value` and now: 0 is today, 1 is tomorrow, negative is
 * in the past. Compared by local calendar day rather than by elapsed hours, so
 * "tomorrow" means the next date on the wall calendar and not 24 hours away.
 *
 * Returns null for a missing or unparseable value, which callers render as a
 * dash instead of a wrong day.
 */
export function daysUntil(value: string | Date | null | undefined): number | null {
  if (!value) return null;
  const date = typeof value === "string" ? new Date(value) : value;
  if (Number.isNaN(date.getTime())) return null;
  const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  return Math.round((startOfDay(date) - startOfDay(new Date())) / 86_400_000);
}
