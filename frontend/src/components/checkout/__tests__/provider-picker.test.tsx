import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { NextIntlClientProvider } from "next-intl";
import messages from "../../../../messages/uz-Latn.json";
import { ProviderPicker } from "../provider-picker";

describe("ProviderPicker", () => {
  it("renders payme and click options and calls onChange when clicked", () => {
    const handleChange = vi.fn();
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <ProviderPicker selected="payme" onChange={handleChange} />
      </NextIntlClientProvider>
    );

    const paymeBtn = screen.getByRole("radio", { name: /payme/i });
    const clickBtn = screen.getByRole("radio", { name: /click/i });

    expect(paymeBtn).toHaveAttribute("aria-checked", "true");
    expect(clickBtn).toHaveAttribute("aria-checked", "false");

    fireEvent.click(clickBtn);
    expect(handleChange).toHaveBeenCalledWith("click");
  });
});
