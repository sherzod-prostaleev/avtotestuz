import Image from "next/image";

type BrandLogoProps = {
  /** Pixel size for the intrinsic width/height (defaults to 36 = h-9). */
  size?: number;
  className?: string;
  /** Empty when the adjacent brand wordmark already names the product. */
  alt?: string;
  priority?: boolean;
};

/**
 * Static Driver Go mark from `/public/logo.svg`.
 * Uses next/image (unoptimized SVG) so chrome logos clear `no-img-element`
 * without touching dynamic MinIO/CDN question media.
 */
export function BrandLogo({
  size = 36,
  className = "rounded-2xl object-cover",
  alt = "",
  priority = false,
}: BrandLogoProps) {
  return (
    <Image
      src="/logo.svg"
      alt={alt}
      width={size}
      height={size}
      className={className}
      unoptimized
      priority={priority}
    />
  );
}
