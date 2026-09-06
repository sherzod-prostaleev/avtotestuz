import type { Metadata } from "next";

// Per-learner certificate lookups: personal, near-duplicate content across
// every code — not something to rank, and not for search results.
export const metadata: Metadata = { robots: { index: false, follow: false } };

export default function SertifikatLayout({ children }: { children: React.ReactNode }) {
  return children;
}
