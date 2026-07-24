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
