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
 * Chrome mark is the compressed 3D PNG (`/logo-512.png`). Tab icons are
 * raster slices of the same render (`favicon.ico` / `favicon-32.png`), not a
 * simplified vector that browsers would prefer over the real mark.
 */
export function BrandLogo({
  size = 36,
  className = "rounded-2xl object-cover",
  alt = "",
  priority = false,
}: BrandLogoProps) {
  return (
    <Image
      src="/logo-512.png"
      alt={alt}
      width={size}
      height={size}
      className={className}
      priority={priority}
    />
  );
}
