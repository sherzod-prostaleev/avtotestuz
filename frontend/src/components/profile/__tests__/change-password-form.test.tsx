import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { afterEach, describe, expect, it, vi } from "vitest";
import messages from "../../../../messages/uz-Latn.json";
import * as apiClient from "@/lib/api-client";
import { ChangePasswordForm } from "../change-password-form";

afterEach(() => {
  vi.restoreAllMocks();
});

function renderForm(props: Parameters<typeof ChangePasswordForm>[0]) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <ChangePasswordForm {...props} />
    </NextIntlClientProvider>
  );
}

describe("ChangePasswordForm for a passwordless (Telegram-created) account", () => {
  it("sets a first password with only the new one, then reports success", async () => {
    const post = vi.spyOn(apiClient, "apiPost").mockResolvedValue({ ok: true });
    const onSuccess = vi.fn();
    renderForm({ hasPassword: false, onSuccess });

    expect(screen.getByText("Parol o'rnatish")).toBeInTheDocument();
    expect(screen.queryByLabelText("Joriy parol")).toBeNull();
    fireEvent.change(screen.getByLabelText("Yangi parol"), { target: { value: "goodpass12" } });
    fireEvent.change(screen.getByLabelText("Yangi parolni tasdiqlash"), { target: { value: "goodpass12" } });
    fireEvent.click(screen.getByRole("button", { name: "Parolni saqlash" }));

    await waitFor(() =>
      expect(post).toHaveBeenCalledWith("me/password/set", { new_password: "goodpass12", confirm_password: "goodpass12" })
    );
    expect(await screen.findByRole("status")).toHaveTextContent("Parol o'rnatildi");
    expect(onSuccess).toHaveBeenCalled();
  });

  it("keeps the ordinary change form (current password) by default", () => {
    renderForm({});
    expect(screen.getByLabelText("Joriy parol")).toBeInTheDocument();
  });
});
