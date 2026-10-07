import { describe, expect, it } from "vitest";
import { cssColorToHex } from "./color";

describe("cssColorToHex", () => {
  it.each([
    // globals.css stores shadcn-style bare HSL triplets.
    ["220 22% 7%", "#0e1116"],
    ["220 16% 96%", "#f3f4f6"],
    ["0 0% 100%", "#ffffff"],
    ["0 0% 0%", "#000000"],
    ["220, 22%, 7%", "#0e1116"],
    ["220deg 22% 7% / 0.5", "#0e1116"],
    ["hsl(220 22% 7%)", "#0e1116"],
    ["hsl(220, 22%, 7%)", "#0e1116"],
    ["#ABCDEF", "#abcdef"],
    ["#fff", "#ffffff"],
    ["rgb(14, 17, 22)", "#0e1116"],
    ["rgb(14 17 22 / 50%)", "#0e1116"],
  ])("%s → %s", (input, hex) => expect(cssColorToHex(input)).toBe(hex));

  it.each(["", "   ", "var(--x)", "red", "220 22%", "#12", "hsl(nope)", "rgb(1,2)"])(
    "rejects %j instead of guessing",
    (input) => expect(cssColorToHex(input)).toBeNull()
  );
});
