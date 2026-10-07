import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { TelegramProvider } from "./telegram-provider";
import { TELEGRAM_SDK_URL, markTelegramMiniApp } from "@/lib/telegram/web-app";

afterEach(() => {
  sessionStorage.clear();
  document.head.querySelectorAll("script").forEach((s) => s.remove());
});

describe("TelegramProvider", () => {
  it("adds no script and renders children on the website", () => {
    render(<TelegramProvider><p>site</p></TelegramProvider>);
    expect(screen.getByText("site")).toBeInTheDocument();
    expect(document.querySelector(`script[src="${TELEGRAM_SDK_URL}"]`)).toBeNull();
  });
  it("injects the SDK once inside the Mini App", () => {
    markTelegramMiniApp();
    const { rerender } = render(<TelegramProvider><p>tg</p></TelegramProvider>);
    rerender(<TelegramProvider><p>tg</p></TelegramProvider>);
    expect(document.querySelectorAll(`script[src="${TELEGRAM_SDK_URL}"]`)).toHaveLength(1);
  });
  it("keeps rendering children when the SDK script fails to load", () => {
    markTelegramMiniApp();
    render(<TelegramProvider><p>still here</p></TelegramProvider>);
    const script = document.querySelector(`script[src="${TELEGRAM_SDK_URL}"]`)!;
    script.dispatchEvent(new Event("error"));
    expect(screen.getByText("still here")).toBeInTheDocument();
  });
});
