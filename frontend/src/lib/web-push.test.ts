import { describe, expect, it } from "vitest";
import { urlBase64ToUint8Array } from "./web-push";

describe("urlBase64ToUint8Array", () => {
  it("decodes URL-safe base64 without padding", () => {
    // "hi" in base64url
    const bytes = urlBase64ToUint8Array("aGk");
    expect(Array.from(bytes)).toEqual([104, 105]);
  });
});
