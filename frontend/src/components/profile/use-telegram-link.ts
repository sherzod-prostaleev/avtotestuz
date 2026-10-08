"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { apiDelete, apiGet, apiPost, ApiError } from "@/lib/api-client";
import { useTelegram } from "@/components/telegram/telegram-provider";
import { linkTelegramInApp } from "@/lib/telegram/auth-body";
import { openExternalUrl } from "@/lib/telegram/links";
import { isLinkedToCurrentUser } from "@/lib/telegram/linked-account";

/** GET /me/telegram. */
export interface TelegramLinkStatus {
  linked: boolean;
  username?: string;
  tg_user_id?: number;
  linked_at?: string;
  /** Linked through a Telegram-signed phone. Older links are not. */
  phone_verified?: boolean;
}

interface LinkTokenResult {
  token: string;
  deep_link: string;
  expires_at: string;
}

export type TelegramLinkErrorKey = "loadError" | "linkError" | "unconfigured" | "linkInAppError" | "unlinkError";

/**
 * What the Mini App shows, from the point of view of the Telegram account
 * that opened it:
 * - "linked": linked to this Telegram account with a verified phone — done.
 * - "confirm": linked to this account, but before phone verification
 *   existed; /tg will not sign in with it until the phone is shared once.
 * - "other": linked to another (or an unidentifiable) Telegram account.
 * - "unlinked": no link at all.
 * Null on the website, or before the status loaded.
 */
export type MiniAppLinkMode = "linked" | "confirm" | "other" | "unlinked";

/**
 * Telegram link state and actions shared by the wide profile card and the
 * phone panel, so the two cannot drift: the website links through a bot deep
 * link; inside the Mini App through Telegram's signed phone share
 * (link-webapp), with no detour out to the bot. Either can unlink.
 */
export function useTelegramLink() {
  const webApp = useTelegram();
  const [status, setStatus] = useState<TelegramLinkStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [errorKey, setErrorKey] = useState<TelegramLinkErrorKey | null>(null);
  const [linking, setLinking] = useState(false);
  const [deepLink, setDeepLink] = useState<string | null>(null);
  const [expiresAt, setExpiresAt] = useState<string | null>(null);
  const [confirmingUnlink, setConfirmingUnlink] = useState(false);
  const [unlinking, setUnlinking] = useState(false);
  // Telegram's sheet answers whenever the learner acts on it; the panel may
  // be gone by then.
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const load = useCallback(async () => {
    setLoading(true);
    setErrorKey(null);
    try {
      const data = await apiGet<TelegramLinkStatus>("me/telegram");
      if (mounted.current) setStatus(data);
    } catch {
      if (mounted.current) setErrorKey("loadError");
    } finally {
      if (mounted.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  /** Website: mint a bot deep link and open it. */
  const startDeepLink = useCallback(async () => {
    setLinking(true);
    setErrorKey(null);
    try {
      const result = await apiPost<LinkTokenResult>("me/telegram/link-token");
      setDeepLink(result.deep_link);
      setExpiresAt(result.expires_at);
      openExternalUrl(result.deep_link);
    } catch (err) {
      setErrorKey(err instanceof ApiError && err.code === "telegram_bot_unconfigured" ? "unconfigured" : "linkError");
    } finally {
      setLinking(false);
    }
  }, []);

  /** Mini App: Telegram's phone sheet, then link-webapp. */
  const linkInApp = useCallback(async () => {
    if (!webApp) return;
    setLinking(true);
    setErrorKey(null);
    const result = await linkTelegramInApp(webApp);
    if (!mounted.current) return;
    setLinking(false);
    if (result === "failed") setErrorKey("linkInAppError");
    if (result === "linked") await load();
  }, [load, webApp]);

  const unlink = useCallback(async () => {
    setUnlinking(true);
    setErrorKey(null);
    try {
      await apiDelete("me/telegram");
      if (!mounted.current) return;
      setConfirmingUnlink(false);
      setDeepLink(null);
      await load();
    } catch {
      if (mounted.current) setErrorKey("unlinkError");
    } finally {
      if (mounted.current) setUnlinking(false);
    }
  }, [load]);

  let mode: MiniAppLinkMode | null = null;
  if (webApp && status) {
    if (!status.linked) mode = "unlinked";
    else if (!isLinkedToCurrentUser(status, webApp)) mode = "other";
    // Missing counts as unverified: an API without the field predates it.
    else mode = status.phone_verified === true ? "linked" : "confirm";
  }

  return {
    inMiniApp: webApp !== null,
    status,
    loading,
    errorKey,
    mode,
    linking,
    deepLink,
    expiresAt,
    confirmingUnlink,
    unlinking,
    load,
    startDeepLink,
    linkInApp,
    unlink,
    askUnlink: () => {
      setErrorKey(null);
      setConfirmingUnlink(true);
    },
    cancelUnlink: () => setConfirmingUnlink(false),
  };
}
