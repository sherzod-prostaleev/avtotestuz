type BrandLogoProps = {
  /** Pixel size for the intrinsic width/height (defaults to 36 = h-9). */
  size?: number;
  className?: string;
  /** Empty when the adjacent brand wordmark already names the product. */
  alt?: string;
  priority?: boolean;
};

/**
 * Chrome mark is a pre-resized WebP. Production `next/image` was serving the
 * full 512×512 PNG (~87KB) for a 36–48px slot (`x-nextjs-cache: MISS`).
 * Tab icons stay the raster slices (`favicon.ico` / `favicon-32.png`).
 */
export function BrandLogo({
  size = 36,
  className = "rounded-2xl object-cover",
  alt = "",
  priority = false,
}: BrandLogoProps) {
  return (
    // Pre-resized WebP. Production next/image served the 87KB PNG for this slot.
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src="/logo-48.webp"
      alt={alt}
      width={size}
      height={size}
      className={className}
      decoding="async"
      fetchPriority={priority ? "high" : "auto"}
    />
  );
}
