import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { AdminBarChart } from "./admin-bar-chart";

describe("AdminBarChart", () => {
  it("shows empty label when no points", () => {
    render(<AdminBarChart points={[]} emptyLabel="no data" />);
    expect(screen.getByText("no data")).toBeInTheDocument();
  });

  it("renders bars for series points", () => {
    const { container } = render(
      <AdminBarChart
        points={[
          { label: "2026-07-01", value: 3 },
          { label: "2026-07-02", value: 0 },
          { label: "2026-07-03", value: 12 },
        ]}
        emptyLabel="empty"
      />,
    );
    expect(container.querySelectorAll("rect").length).toBe(3);
    expect(screen.getByRole("img", { name: "chart" })).toBeInTheDocument();
  });
});
