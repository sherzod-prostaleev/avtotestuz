/**
 * Fades a page in on navigation.
 *
 * Used as the default export of each route group's `template.tsx`. Next
 * re-mounts a template on navigation (a layout it would keep), so React builds
 * a fresh DOM node and the `.page-fade` animation in globals.css restarts —
 * no client JS, no animation library.
 *
 * It belongs inside each route group rather than at `[locale]`, for two
 * reasons. Higher up, React reuses the same node and the animation never
 * replays. And in `(app)` the shell — sidebar, bottom nav — lives in the
 * layout above this, so it stays mounted while only the page content fades.
 *
 * A plain block box, not `display: contents`: an element that generates no box
 * never paints, and so cannot animate its opacity.
 */
export function PageFade({ children }: { children: React.ReactNode }) {
  return <div className="page-fade">{children}</div>;
}
