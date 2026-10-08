// A button label has room for one short word next to "sifatida davom etish";
// the heading above it keeps the full name.
const MAX_CHARS = 16;

/** The name to put on "continue as …", or null when there is none to show. */
export function continueName(firstName: string): string | null {
  const word = firstName.trim().split(/\s+/)[0] ?? "";
  if (!word) return null;
  // Array.from counts code points, so an emoji is never cut in half.
  const chars = Array.from(word);
  return chars.length > MAX_CHARS ? `${chars.slice(0, MAX_CHARS).join("")}…` : word;
}
