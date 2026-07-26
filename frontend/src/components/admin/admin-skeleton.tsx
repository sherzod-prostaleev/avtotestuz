"use client";

type AdminSkeletonProps = {
  className?: string;
  rows?: number;
};

export function AdminSkeleton({ className = "", rows = 4 }: AdminSkeletonProps) {
  return (
    <div className={`space-y-2 ${className}`} aria-hidden>
      {Array.from({ length: rows }).map((_, i) => (
        <div
          key={i}
          className="h-10 animate-pulse rounded-xl bg-white/[0.06]"
          style={{ opacity: 1 - i * 0.12 }}
        />
      ))}
    </div>
  );
}

export function AdminSkeletonTiles({ count = 6 }: { count?: number }) {
  return (
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3" aria-hidden>
      {Array.from({ length: count }).map((_, i) => (
        <div key={i} className="h-24 animate-pulse rounded-2xl bg-white/[0.06]" />
      ))}
    </div>
  );
}
