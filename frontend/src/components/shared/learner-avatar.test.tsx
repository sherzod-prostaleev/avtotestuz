import { fireEvent, render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LearnerAvatar } from "./learner-avatar";

const PHOTO = "https://drivergo.uz/media/images/avatars/abcdefghijklmnopqrstuvwxyz.jpg";
const SIZE = "flex h-11 w-11 items-center justify-center rounded-full";

describe("LearnerAvatar", () => {
  it("shows the photo when the learner has one", () => {
    const { container } = render(<LearnerAvatar name="Ali" src={PHOTO} className={SIZE} />);
    const img = container.querySelector("img");
    expect(img).not.toBeNull();
    expect(img?.getAttribute("src")).toBe(PHOTO);
    // Decorative: the name is always printed next to it.
    expect(img?.getAttribute("alt")).toBe("");
    expect(img?.getAttribute("loading")).toBe("lazy");
    expect(img?.getAttribute("decoding")).toBe("async");
    expect(img?.className).toContain("object-cover");
  });

  it("keeps the circle's own size classes so nothing shifts", () => {
    const { container } = render(<LearnerAvatar name="Ali" src={PHOTO} className={SIZE} />);
    const box = container.firstElementChild as HTMLElement;
    for (const cls of SIZE.split(" ")) expect(box.className).toContain(cls);
    expect(box.className).toContain("overflow-hidden");
  });

  it("falls back to the initial when the photo fails to load", () => {
    const { container, getByText } = render(<LearnerAvatar name="ali" src={PHOTO} className={SIZE} />);
    fireEvent.error(container.querySelector("img")!);
    expect(container.querySelector("img")).toBeNull();
    expect(getByText("A")).toBeInTheDocument();
  });

  it("tries again when a different photo URL arrives", () => {
    const { container, rerender } = render(<LearnerAvatar name="Ali" src={PHOTO} className={SIZE} />);
    fireEvent.error(container.querySelector("img")!);
    rerender(<LearnerAvatar name="Ali" src={PHOTO.replace("abc", "xyz")} className={SIZE} />);
    expect(container.querySelector("img")).not.toBeNull();
  });

  it("draws the initial when there is no photo", () => {
    for (const src of [undefined, null, ""]) {
      const { container, getByText, unmount } = render(<LearnerAvatar name=" zarina" src={src} className={SIZE} />);
      expect(container.querySelector("img")).toBeNull();
      expect(getByText("Z")).toBeInTheDocument();
      unmount();
    }
  });

  it("uses a whole character for the initial, not half of an emoji", () => {
    const { container } = render(<LearnerAvatar name="😀 Ali" className={SIZE} />);
    expect(container.textContent).toBe("😀");
  });
});
