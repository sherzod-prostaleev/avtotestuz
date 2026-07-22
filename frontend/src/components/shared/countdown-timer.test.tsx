import { render, screen, act } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import { CountdownTimer } from "./countdown-timer";

describe("CountdownTimer", () => {
  it("formats minutes and seconds with zero-padding", () => {
    render(<CountdownTimer remainingSeconds={125} />);
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("2:05");
  });

  it("uses the gold color when more than a minute remains", () => {
    render(<CountdownTimer remainingSeconds={90} />);
    expect(screen.getByTestId("countdown-timer").className).toContain("text-gold");
  });

  it("switches to a pulsating red state in the last minute", () => {
    render(<CountdownTimer remainingSeconds={45} />);
    const el = screen.getByTestId("countdown-timer");
    expect(el.className).toContain("text-danger");
    expect(el.className).toContain("animate-pulse");
  });

  it("never shows a negative time", () => {
    render(<CountdownTimer remainingSeconds={-5} />);
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("0:00");
  });
});

describe("CountdownTimer ticking", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("decrements the displayed time by one each real second", () => {
    vi.useFakeTimers();
    render(<CountdownTimer remainingSeconds={10} />);
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("0:10");

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("0:09");

    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("0:06");
  });

  it("calls onExpire exactly once when reaching zero, and never again afterward", () => {
    vi.useFakeTimers();
    const onExpire = vi.fn();
    render(<CountdownTimer remainingSeconds={3} onExpire={onExpire} />);

    act(() => {
      vi.advanceTimersByTime(3000);
    });
    expect(onExpire).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("0:00");

    act(() => {
      vi.advanceTimersByTime(10_000);
    });
    expect(onExpire).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("0:00");
  });

  it("re-syncs the displayed value when the remainingSeconds prop changes after mount", () => {
    vi.useFakeTimers();
    const { rerender } = render(<CountdownTimer remainingSeconds={100} />);

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("1:38");

    rerender(<CountdownTimer remainingSeconds={50} />);
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("0:50");

    act(() => {
      vi.advanceTimersByTime(1000);
    });
    expect(screen.getByTestId("countdown-timer")).toHaveTextContent("0:49");
  });
});
