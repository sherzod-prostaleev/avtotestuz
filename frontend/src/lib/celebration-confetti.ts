import confetti from "canvas-confetti";

/** Matches backend `session.ExamErrorsAllowed` for 20-question exam/bilet scoring. */
export const EXAM_ERRORS_ALLOWED = 2;

/** Pass bar used by the real exam (and bilet praise): ≥18/20, ≤2 wrong. */
export function meetsExamPassThreshold(correct: number, total: number): boolean {
  if (total <= 0) return false;
  const wrong = Math.max(0, total - correct);
  return correct >= total - EXAM_ERRORS_ALLOWED && wrong <= EXAM_ERRORS_ALLOWED;
}

function prefersReducedMotion(): boolean {
  if (typeof window === "undefined") return true;
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}

const BRAND_COLORS = ["#f59e0b", "#22c55e", "#eab308", "#38bdf8", "#f8fafc"];

/**
 * Side-cannon salute used for full exam-pass theater.
 * No-ops when the user prefers reduced motion.
 */
export function fireExamPassSalute(durationMs = 2800): () => void {
  if (prefersReducedMotion()) return () => undefined;

  const end = Date.now() + durationMs;
  let raf = 0;
  let cancelled = false;

  const frame = () => {
    if (cancelled) return;
    confetti({
      particleCount: 4,
      angle: 60,
      spread: 58,
      startVelocity: 48,
      origin: { x: 0, y: 0.72 },
      colors: BRAND_COLORS,
      disableForReducedMotion: true,
    });
    confetti({
      particleCount: 4,
      angle: 120,
      spread: 58,
      startVelocity: 48,
      origin: { x: 1, y: 0.72 },
      colors: BRAND_COLORS,
      disableForReducedMotion: true,
    });
    if (Date.now() < end) {
      raf = requestAnimationFrame(frame);
    } else {
      // Closing skyburst — one memorable beat, not endless noise.
      confetti({
        particleCount: 90,
        spread: 100,
        startVelocity: 42,
        origin: { x: 0.5, y: 0.35 },
        colors: BRAND_COLORS,
        disableForReducedMotion: true,
      });
    }
  };

  frame();
  return () => {
    cancelled = true;
    if (raf) cancelAnimationFrame(raf);
  };
}

/** Short, quieter burst for bilet praise — not the exam fireworks show. */
export function fireBiletPraiseBurst(): void {
  if (prefersReducedMotion()) return;
  confetti({
    particleCount: 42,
    spread: 70,
    startVelocity: 28,
    origin: { x: 0.5, y: 0.55 },
    colors: ["#f59e0b", "#eab308", "#22c55e"],
    disableForReducedMotion: true,
  });
}
