import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useTelegramBackButton } from "./use-telegram-back-button";

const tg = vi.hoisted(() => ({ webApp: null as null | Record<string, unknown> }));
vi.mock("./telegram-provider", () => ({ useTelegram: () => tg.webApp }));

function fakeBack() {
  const handlers = new Set<() => void>();
  const BackButton = {
    show: vi.fn(),
    hide: vi.fn(),
    onClick: vi.fn((cb: () => void) => handlers.add(cb)),
    offClick: vi.fn((cb: () => void) => handlers.delete(cb)),
  };
  tg.webApp = { BackButton };
  return { BackButton, press: () => handlers.forEach((cb) => cb()), handlers };
}

afterEach(() => {
  tg.webApp = null;
});

describe("useTelegramBackButton", () => {
  it("shows Back while mounted and sends it to the latest callback", () => {
    const back = fakeBack();
    const first = vi.fn();
    const second = vi.fn();
    const view = renderHook(({ cb }) => useTelegramBackButton(cb), { initialProps: { cb: first } });
    expect(back.BackButton.show).toHaveBeenCalledTimes(1);
    view.rerender({ cb: second });
    back.press();
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
    view.unmount();
    expect(back.BackButton.hide).toHaveBeenCalled();
    expect(back.handlers.size).toBe(0);
  });

  it("does nothing on the website", () => {
    expect(() => renderHook(() => useTelegramBackButton(vi.fn()))).not.toThrow();
  });
});
