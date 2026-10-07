import { describe, expect, it } from "vitest";
import { isLinkedToCurrentUser } from "./linked-account";

const app = (username?: string) => ({ initDataUnsafe: { user: { id: 1, username } } });

describe("isLinkedToCurrentUser", () => {
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
  ] as const)("%s", (_name, linked, webApp, want) => {
    expect(isLinkedToCurrentUser(linked, webApp)).toBe(want);
  });
});
