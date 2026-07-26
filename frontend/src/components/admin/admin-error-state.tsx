"use client";

import { Button } from "@/components/ui/button";

type AdminErrorStateProps = {
  message: string;
  retryLabel?: string;
  onRetry?: () => void;
};

export function AdminErrorState({ message, retryLabel, onRetry }: AdminErrorStateProps) {
  return (
    <div
      role="alert"
      className="rounded-2xl border border-destructive/40 bg-destructive/10 px-4 py-3 text-sm text-destructive"
    >
      <p>{message}</p>
      {onRetry && retryLabel ? (
        <Button type="button" size="sm" variant="outline" className="mt-3" onClick={onRetry}>
          {retryLabel}
        </Button>
      ) : null}
    </div>
  );
}
