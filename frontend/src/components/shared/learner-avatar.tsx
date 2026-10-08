"use client";

import { useState } from "react";

interface LearnerAvatarProps {
  /** The learner's name (or whatever the spot showed before); its first character is the fallback. */
  name: string;
  /** `profile.avatar_url` from /me: present only for a phone-verified Telegram link. */
  src?: string | null;
  /** The spot's own size, shape and colours — the same classes the initial bubble always had. */
  className: string;
}

/** First user-perceived character, so an emoji or a surrogate pair is not cut in half. */
function initialOf(name: string): string {
  const first = Array.from(name.trim())[0];
  return first ? first.toUpperCase() : "";
}

/**
 * The learner's round avatar: their Telegram photo when /me has one, the
 * initial letter otherwise.
 *
 * The letter is always rendered and the photo is laid over it, so the box is
 * the same size either way (no layout shift), a slow photo shows the letter
 * instead of an empty circle, and a photo that fails — deleted, blocked,
 * offline — simply goes away. A failure is remembered per URL: a refreshed
 * photo arrives under a new URL and is tried again.
 */
export function LearnerAvatar({ name, src, className }: LearnerAvatarProps) {
  const [failedSrc, setFailedSrc] = useState<string | null>(null);
  const photo = src && src !== failedSrc ? src : null;

  return (
    <span suppressHydrationWarning className={`relative overflow-hidden ${className}`}>
      <span aria-hidden={photo ? true : undefined}>{initialOf(name)}</span>
      {photo && (
        // A 256px JPEG from our own /media: next/image would add a resize
        // round trip for nothing.
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={photo}
          alt=""
          loading="eager"
          decoding="async"
          draggable={false}
          onError={() => setFailedSrc(photo)}
          className="absolute inset-0 h-full w-full object-cover"
        />
      )}
    </span>
  );
}
