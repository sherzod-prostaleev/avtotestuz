import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import type { ReactNode } from "react";

interface BackLinkProps {
  href: string;
  /**
   * On the kiosk this is the only way out of a page: a classroom PC has no
   * sidebar, no browser chrome and no mouse — a student taps the screen. The
   * learner app keeps the quiet text link, because the sidebar is right there.
   */
  kiosk?: boolean;
  children: ReactNode;
}

export function BackLink({ href, kiosk = false, children }: BackLinkProps) {
  return (
    <Link href={href} className={kiosk ? "back-link-kiosk" : "back-link"}>
      <ArrowLeft aria-hidden="true" className={kiosk ? "h-5 w-5 shrink-0" : "h-4 w-4"} />
      {children}
    </Link>
  );
}
