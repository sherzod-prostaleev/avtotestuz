import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { ThemeProvider } from "next-themes";
import { ThemeToggle } from "./theme-toggle";

function renderWithTheme() {
  return render(
    <ThemeProvider attribute="class" defaultTheme="dark" enableSystem={false}>
      <ThemeToggle />
    </ThemeProvider>
  );
}

describe("ThemeToggle", () => {
  it("shows the sun icon (offering to switch to light) while the theme is dark", async () => {
    renderWithTheme();
    await waitFor(() => expect(screen.getByTestId("theme-toggle-sun")).toBeInTheDocument());
  });

  it("switches to the moon icon after being clicked", async () => {
    renderWithTheme();
    await waitFor(() => expect(screen.getByRole("button")).toBeInTheDocument());
    fireEvent.click(screen.getByRole("button"));
    await waitFor(() => expect(screen.getByTestId("theme-toggle-moon")).toBeInTheDocument());
  });
});
