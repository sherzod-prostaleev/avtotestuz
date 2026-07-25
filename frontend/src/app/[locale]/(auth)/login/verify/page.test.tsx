import { render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { describe, it, expect, vi, afterEach } from "vitest";
import messages from "../../../../../../messages/uz-Latn.json";
import VerifyPage from "./page";

const replaceMock = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock }),
}));

afterEach(() => {
  replaceMock.mockClear();
});

describe("VerifyPage", () => {
  it("redirects legacy OTP verify links to login", async () => {
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <VerifyPage />
      </NextIntlClientProvider>
    );
    expect(screen.getByRole("status")).toBeInTheDocument();
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/uz-Latn/login"));
  });
});
