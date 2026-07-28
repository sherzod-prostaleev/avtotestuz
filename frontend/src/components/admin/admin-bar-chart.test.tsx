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

  it("shows the empty label when only one day carries data", () => {
    const points = Array.from({ length: 14 }, (_, i) => ({
      label: `2026-07-${String(i + 15).padStart(2, "0")}`,
      value: i === 13 ? 3 : 0,
    }));
    render(<AdminBarChart points={points} emptyLabel="Ma’lumot yetarli emas" />);
    expect(screen.getByText("Ma’lumot yetarli emas")).toBeInTheDocument();
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
  });

  it("renders the chart once two or more days carry data", () => {
    const points = [
      { label: "2026-07-27", value: 3 },
      { label: "2026-07-28", value: 5 },
    ];
    render(<AdminBarChart points={points} emptyLabel="Ma’lumot yetarli emas" />);
    expect(screen.queryByText("Ma’lumot yetarli emas")).not.toBeInTheDocument();
    expect(screen.getByRole("img")).toBeInTheDocument();
  });
});
