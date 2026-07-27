import { describe, expect, it } from "vitest";
import { encodeClient, isPlayableQuestion, medalLabel, parseEnvelope } from "@/lib/arena-protocol";

describe("arena-protocol", () => {
  it("round-trips encode/parse", () => {
    const raw = encodeClient("queue.join", { locale: "uz-Latn" });
    const env = parseEnvelope(raw);
    expect(env.v).toBe(1);
    expect(env.t).toBe("queue.join");
    expect((env.d as { locale: string }).locale).toBe("uz-Latn");
  });

  it("rejects bad protocol version", () => {
    expect(() => parseEnvelope(JSON.stringify({ v: 99, t: "hello", d: {} }))).toThrow(
      /bad_protocol/
    );
  });

  it("maps medal labels", () => {
    expect(medalLabel("bronze")).toBe("Bronze");
    expect(medalLabel("brilliant")).toBe("Brilliant");
  });

  it("rejects incomplete question payloads that used to crash Arena", () => {
    expect(isPlayableQuestion({ id: "q1" })).toBe(false);
    expect(isPlayableQuestion({ id: "q1", answers: [] })).toBe(false);
    expect(isPlayableQuestion({ id: "q1", answers: null })).toBe(false);
    expect(
      isPlayableQuestion({
        id: "q1",
        text: "Savol?",
        answers: [{ id: "a1", position: 1, text: "A" }],
      })
    ).toBe(true);
  });
});
