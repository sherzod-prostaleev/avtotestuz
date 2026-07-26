import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { MasteryBar } from "./mastery-bar";

describe("MasteryBar", () => {
  it("colors the fill red below 40%", () => {
    render(<MasteryBar categoryName="Chorrahalar" masteryPercent={25} />);
    expect(screen.getByTestId("mastery-bar-fill").className).toContain("bg-danger");
  });

  it("colors the fill gold between 40% and 79%", () => {
    render(<MasteryBar categoryName="Chorrahalar" masteryPercent={55} />);
    expect(screen.getByTestId("mastery-bar-fill").className).toContain("bg-gold");
  });

  it("colors the fill green at 80% and above", () => {
    render(<MasteryBar categoryName="Chorrahalar" masteryPercent={92} />);
    expect(screen.getByTestId("mastery-bar-fill").className).toContain("bg-success");
  });

  it("clamps the rendered width to 0-100", () => {
    render(<MasteryBar categoryName="Chorrahalar" masteryPercent={150} />);
    expect(screen.getByTestId("mastery-bar-fill")).toHaveStyle({ width: "100%" });
  });

  it("shows studied/total coverage next to the percent", () => {
    render(<MasteryBar categoryName="Chorrahalar" masteryPercent={12} studied={4} total={138} />);
    expect(screen.getByText("12%")).toBeInTheDocument();
    expect(screen.getByText("4/138")).toBeInTheDocument();
  });
});
