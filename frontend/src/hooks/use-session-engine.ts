import { useState, useCallback } from "react";
import { apiGet, apiPost, ApiError } from "@/lib/api-client";
import { defaultLocale } from "@/i18n/config";

export type SessionMode = "variant" | "exam" | "practice" | "mistakes";

export interface AnswerOptionItem {
  id: string;
  text: string;
  image_url?: string | null;
}

export interface QuestionExplanationBlock {
  type: "intro" | "important" | "warning" | "tip" | "option_analysis" | "summary";
  content: string;
}

export interface QuestionExplanation {
  blocks: QuestionExplanationBlock[];
}

/**
 * A question as rendered during a session.
 *
 * IMPORTANT (anti-cheat): `correct` and `correct_answer_id` are NEVER populated
 * from the content API (which by design never returns them). They are ONLY ever
 * copied verbatim from a `submitAnswer` / `loadSession` backend response, and
 * stay `undefined` until the backend chooses to send them (e.g. in exam mode
 * they only appear after the session ends). They must never be computed,
 * defaulted, or inferred on the client.
 */
export interface SessionQuestionItem {
  id: string;
  question: string;
  image_url?: string | null;
  answers: AnswerOptionItem[];
  user_answer_id?: string | null;
  /** whether this question has been answered (from resume; the chosen answer id
   * itself is not part of the backend resume contract). */
  answered?: boolean;
  /** correctness, straight from the backend — undefined until it sends it. */
  correct?: boolean;
  /** the id of the correct answer, straight from the backend — undefined until sent. */
  correct_answer_id?: string;
  explanation?: QuestionExplanation | null;
}

export interface SessionState {
  id: string;
  mode: SessionMode;
  time_limit_sec: number | null;
  remaining_sec: number | null;
  status: "active" | "completed";
  questions: SessionQuestionItem[];
  score: number | null;
  total: number | null;
  stopped_reason: string | null;
  passed: boolean | null;
  completed_at: string | null;
}

/** Typed error surfaced to the pages so they can branch on `.code`. */
export interface SessionError {
  code: string;
  message: string;
}

export interface StartSessionOptions {
  variant_id?: number | string;
  category_id?: string;
  sign_id?: string;
  question_count?: number;
  locale?: string;
}

interface StartSessionResponse {
  id: string;
  mode: SessionMode;
  question_ids: string[];
  time_limit_sec: number | null;
  total: number;
  started_at: string;
}

interface QuestionDetailResponse {
  id: string;
  category_code: string;
  text: string;
  image_url: string | null;
  answers: { id: string; position: number; text: string; image_url: string | null }[];
  signs: { code: string; name: string; image_url: string | null }[];
  explanation: { legal_refs: unknown; blocks: unknown } | null;
}

export interface SubmitAnswerResponse {
  recorded: boolean;
  correct?: boolean;
  correct_answer_id?: string;
  stopped?: boolean;
  stop_reason?: string;
}

interface FinishSessionResponse {
  status: "passed" | "failed" | "abandoned";
  stopped_reason: "completed" | "time_up" | "too_many_errors";
  score: number;
  total: number;
}

interface SessionDetailResponse {
  id: string;
  mode: SessionMode;
  total: number;
  status: "in_progress" | "passed" | "failed" | "abandoned";
  stopped_reason: string;
  score?: number;
  started_at: string;
  finished_at?: string;
  answers: { question_id: string; position: number; answered: boolean; correct?: boolean }[];
}

/** Normalize any thrown value into the typed SessionError we expose to pages. */
function toSessionError(err: unknown): SessionError {
  if (err instanceof ApiError) {
    return { code: err.code, message: err.message };
  }
  // A thrown non-ApiError means `fetch` itself rejected — no HTTP response.
  return { code: "network_error", message: "Tarmoq xatosi. Internetni tekshiring." };
}

/**
 * Narrow the genuinely-unknown backend explanation into the renderable shape the
 * QuestionCard consumes. We render ONLY structurally-present blocks and never
 * fabricate content; anything that does not match is dropped to `null`.
 */
function narrowExplanation(raw: QuestionDetailResponse["explanation"]): QuestionExplanation | null {
  if (!raw || typeof raw !== "object") return null;
  const blocks = (raw as { blocks?: unknown }).blocks;
  if (!Array.isArray(blocks)) return null;
  const parsed: QuestionExplanationBlock[] = [];
  for (const b of blocks) {
    if (b && typeof b === "object" && typeof (b as { content?: unknown }).content === "string") {
      const type = (b as { type?: unknown }).type;
      parsed.push({
        type: (typeof type === "string" ? type : "intro") as QuestionExplanationBlock["type"],
        content: (b as { content: string }).content,
      });
    }
  }
  return parsed.length > 0 ? { blocks: parsed } : null;
}

/** Build a SessionQuestionItem from the content API's QuestionDetail. */
function toQuestionItem(d: QuestionDetailResponse): SessionQuestionItem {
  return {
    id: d.id,
    question: d.text,
    image_url: d.image_url,
    answers: d.answers.map((a) => ({ id: a.id, text: a.text, image_url: a.image_url })),
    explanation: narrowExplanation(d.explanation),
  };
}

/** Fetch each question's public content, in the given order. */
async function fetchQuestions(ids: string[], locale: string): Promise<SessionQuestionItem[]> {
  const details = await Promise.all(
    ids.map((id) => apiGet<QuestionDetailResponse>(`questions/${id}?locale=${encodeURIComponent(locale)}`))
  );
  return details.map(toQuestionItem);
}

export function useSessionEngine(_initialSessionId?: string) {
  const [session, setSession] = useState<SessionState | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [error, setError] = useState<SessionError | null>(null);

  const startSession = useCallback(
    async (mode: SessionMode, options?: StartSessionOptions): Promise<SessionState | null> => {
      setLoading(true);
      setError(null);
      const locale = options?.locale ?? defaultLocale;
      try {
        const payload: Record<string, unknown> = { mode };
        if (options?.variant_id !== undefined && options.variant_id !== null) {
          payload.variant_id = String(options.variant_id);
        }
        if (options?.category_id !== undefined && options.category_id !== null) {
          payload.category_id = String(options.category_id);
        }
        if (options?.sign_id !== undefined && options.sign_id !== null) {
          payload.sign_id = String(options.sign_id);
        }
        if (options?.question_count !== undefined && options.question_count !== null) {
          payload.count = options.question_count;
        }
        if (options?.locale) {
          payload.locale = options.locale;
        }

        const created = await apiPost<StartSessionResponse>("sessions", payload);
        const questions = await fetchQuestions(created.question_ids, locale);

        const state: SessionState = {
          id: created.id,
          mode: created.mode,
          time_limit_sec: created.time_limit_sec,
          remaining_sec: created.time_limit_sec,
          status: "active",
          questions,
          score: null,
          total: created.total,
          stopped_reason: null,
          passed: null,
          completed_at: null,
        };
        setSession(state);
        return state;
      } catch (err: unknown) {
        // Never fabricate a session. Surface the typed error; leave session null.
        setError(toSessionError(err));
        return null;
      } finally {
        setLoading(false);
      }
    },
    []
  );

  const loadSession = useCallback(
    async (sessionId: string, locale?: string): Promise<SessionState | null> => {
      setLoading(true);
      setError(null);
      const loc = locale ?? defaultLocale;
      try {
        const detail = await apiGet<SessionDetailResponse>(`sessions/${sessionId}`);
        const ordered = [...detail.answers].sort((a, b) => a.position - b.position);
        const questions = await fetchQuestions(
          ordered.map((a) => a.question_id),
          loc
        );
        // Mark answered/correct from the resume payload. The chosen answer id is
        // NOT part of the resume contract, so user_answer_id stays unset.
        const merged = questions.map((q, i) => ({
          ...q,
          answered: ordered[i].answered,
          correct: ordered[i].correct,
        }));

        const completed = detail.status !== "in_progress";
        const state: SessionState = {
          id: detail.id,
          mode: detail.mode,
          // The resume contract does not include the time limit, so remaining
          // time cannot be reconstructed reliably — leave it null.
          time_limit_sec: null,
          remaining_sec: null,
          status: completed ? "completed" : "active",
          questions: merged,
          score: detail.score ?? null,
          total: detail.total,
          stopped_reason: detail.stopped_reason || null,
          passed: completed ? detail.status === "passed" : null,
          completed_at: detail.finished_at ?? null,
        };
        setSession(state);
        return state;
      } catch (err: unknown) {
        setError(toSessionError(err));
        return null;
      } finally {
        setLoading(false);
      }
    },
    []
  );

  const submitAnswer = useCallback(
    async (
      sessionId: string,
      questionId: string,
      answerId: string
    ): Promise<SubmitAnswerResponse | null> => {
      setSubmitting(true);
      setError(null);
      try {
        const resp = await apiPost<SubmitAnswerResponse>(`sessions/${sessionId}/answers`, {
          question_id: questionId,
          answer_id: answerId,
        });
        // Update ONLY this question, copying correctness verbatim from the
        // backend (undefined stays undefined — never computed client-side).
        setSession((prev) => {
          if (!prev) return prev;
          return {
            ...prev,
            questions: prev.questions.map((q) =>
              q.id === questionId
                ? {
                    ...q,
                    user_answer_id: answerId,
                    answered: true,
                    correct: resp.correct,
                    correct_answer_id: resp.correct_answer_id,
                  }
                : q
            ),
          };
        });
        return resp;
      } catch (err: unknown) {
        // Do NOT touch local answer state — no client-computed correctness.
        setError(toSessionError(err));
        return null;
      } finally {
        setSubmitting(false);
      }
    },
    []
  );

  const finishSession = useCallback(
    async (sessionId: string): Promise<SessionState | null> => {
      setLoading(true);
      setError(null);
      try {
        const resp = await apiPost<FinishSessionResponse>(`sessions/${sessionId}/finish`);
        let next: SessionState | null = null;
        setSession((prev) => {
          if (!prev) return prev;
          next = {
            ...prev,
            status: "completed",
            score: resp.score,
            total: resp.total,
            stopped_reason: resp.stopped_reason,
            passed: resp.status === "passed",
            completed_at: prev.completed_at ?? new Date().toISOString(),
          };
          return next;
        });
        return next;
      } catch (err: unknown) {
        // Do NOT fake completion.
        setError(toSessionError(err));
        return null;
      } finally {
        setLoading(false);
      }
    },
    []
  );

  return {
    session,
    loading,
    submitting,
    error,
    loadSession,
    startSession,
    submitAnswer,
    finishSession,
  };
}
