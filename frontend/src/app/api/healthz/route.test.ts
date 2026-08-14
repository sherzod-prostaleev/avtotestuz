import { describe, expect, it } from "vitest";
import { GET } from "./route";

describe("web healthz", () => {
  it("returns a tiny no-store 200 without touching the backend", async () => {
    const response = GET();
    expect(response.status).toBe(200);
    expect(response.headers.get("cache-control")).toBe("no-store");
    expect(await response.text()).toBe("ok");
  });
});
