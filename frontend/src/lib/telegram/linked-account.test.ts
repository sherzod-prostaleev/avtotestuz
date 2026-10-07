import { describe, expect, it } from "vitest";
import { isLinkedToCurrentUser } from "./linked-account";

const app = (username?: string, id = 1) => ({ initDataUnsafe: { user: { id, username } } });

describe("isLinkedToCurrentUser", () => {
  // The id is what Telegram signs and never changes; it decides when known.
  it.each([
    ["same id, different usernames", { tg_user_id: 1, username: "old_name" }, app("new_name"), true],
    ["different id, same username", { tg_user_id: 2, username: "sherzod" }, app("sherzod"), false],
    ["same id, no usernames", { tg_user_id: 1 }, app(undefined), true],
    ["linked id but no launch user", { tg_user_id: 1, username: "sherzod" }, { initDataUnsafe: {} }, false],
  ] as const)("by id: %s", (_name, linked, webApp, want) => {
    expect(isLinkedToCurrentUser(linked, webApp)).toBe(want);
  });

  // Fallback for a response without tg_user_id (older API).
  it.each([
    ["same username", "sherzod", app("sherzod"), true],
    ["case and @ ignored", "@Sherzod", app("sherZOD"), true],
    ["different account", "aziz", app("sherzod"), false],
    ["no linked username", undefined, app("sherzod"), false],
    ["no current username", "sherzod", app(undefined), false],
    ["both missing", undefined, app(undefined), false],
    ["blank linked username", " ", app(" "), false],
    ["no launch user", "sherzod", { initDataUnsafe: {} }, false],
    ["no web app", "sherzod", null, false],
  ] as const)("by username: %s", (_name, username, webApp, want) => {
    expect(isLinkedToCurrentUser({ username }, webApp)).toBe(want);
  });

  it("is false without a status", () => {
    expect(isLinkedToCurrentUser(null, app("sherzod"))).toBe(false);
  });
});
