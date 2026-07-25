import Link from "next/link";

type OpsDeprecatedBannerProps = {
  locale: string;
  href: string;
  label: string;
  note: string;
};

/** Points operators to the Admin SoT surface; ops remains token escape hatch. */
export function OpsDeprecatedBanner({ locale, href, label, note }: OpsDeprecatedBannerProps) {
  const target = href.startsWith("/") ? href : `/${locale}/admin/${href}`;
  return (
    <p
      role="note"
      className="mb-4 rounded-xl border border-border bg-card px-4 py-3 text-sm text-muted-foreground"
    >
      <span className="font-semibold text-foreground">{note}</span>{" "}
      <Link href={target} className="font-semibold text-accent underline-offset-2 hover:underline">
        {label}
      </Link>
    </p>
  );
}
