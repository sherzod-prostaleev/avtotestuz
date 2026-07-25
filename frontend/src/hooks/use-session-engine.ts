import { useCallback, useRef, useState } from "react";
import { apiGet, apiPost, ApiError } from "@/lib/api-client";
import { defaultLocale } from "@/i18n/config";

export type SessionMode = "variant" | "exam" | "practice" | "mistakes" | "grand_mock" | "review";

export interface AnswerOptionItem {
  id: string;
  text: string;
  image_url?: string | null;
  position?: number;
}

export interface QuestionExplanationItem {
  position: number;
  correct: boolean;
  text: string;
}

/**
 * Renderable explanation block.
 *
 * The canonical backend shape is `{ type, text, items? }`. `content` is a
 * compatibility projection consumed by QuestionCard; it is made only from
 * the backend's text/items and never contains generated explanation prose.
 */
export interface QuestionExplanationBlock {
  type: string;
  content: string;
  text?: string;
  items?: QuestionExplanationItem[];
}

export interface QuestionExplanation {
  legal_refs?: unknown;
  blocks: QuestionExplanationBlock[];
}

/**
 * A question as rendered during a session.
 *
 * IMPORTANT (anti-cheat): correctness is copied only from authenticated
 * session responses. It is never fetched from a public content endpoint and
 * is never inferred or calculated in the browser.
 */
export interface SessionQuestionItem {
  id: string;
  question: string;
  image_url?: string | null;
  answers: AnswerOptionItem[];
  position?: number;
  user_answer_id?: string | null;
  answered?: boolean;
  correct?: boolean;
  correct_answer_id?: string;
  explanation?: QuestionExplanation | null;
}

export interface SessionState {
  id: string;
  mode: SessionMode;
  time_limit_sec: number | null;
  remaining_sec: number | null;
  status: "active" | "completed" | "result_pending";
  questions: SessionQuestionItem[];
  score: number | null;
  total: number | null;
  stopped_reason: string | null;
  passed: boolean | null;
  completed_at: string | null;
  /** Present after a passed Grand Mock when the backend persisted a shareable id. */
  certificate_share_code?: string | null;
}

/** Typed error surfaced to pages so they can branch on `.code`. */
export interface SessionError {
  code: string;
  message: string;
}

export interface StartSessionOptions {
  variant_id?: number | string;
  category_id?: string;
  sign_id?: string;
  /** Practice selector: true = illustrated questions only, false = text-only. */
  has_image?: boolean;
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

interface RawExplanationPayload {
  legal_refs: unknown;
  blocks: unknown;
}

interface QuestionDetailResponse {
  id: string;
  category_code: string;
  text: string;
  image_url: string | null;
  answers: { id: string; position: number; text: string; image_url: string | null }[];
  signs: { code: string; name: string; image_url: string | null }[];
  explanation: RawExplanationPayload | null;
  position: number;
  answered: boolean;
  user_answer_id?: string | null;
  correct?: boolean;
  correct_answer_id?: string | null;
}

export interface SubmitAnswerResponse {
  recorded: boolean;
  correct?: boolean;
  correct_answer_id?: string;
  explanation?: RawExplanationPayload | null;
  stopped?: boolean;
  stop_reason?: string;
}

interface FinishSessionResponse {
  status: "passed" | "failed" | "abandoned";
  stopped_reason: string;
  score: number;
  total: number;
  certificate_share_code?: string;
}

interface SessionAnswerResponse {
  question_id: string;
  position: number;
  answered: boolean;
  user_answer_id?: string | null;
  correct?: boolean;
  correct_answer_id?: string | null;
}

interface SessionDetailResponse {
  id: string;
  mode: SessionMode;
  total: number;
  status: "in_progress" | "passed" | "failed" | "abandoned";
  stopped_reason: string;
  score?: number;
  time_limit_sec?: number | null;
  started_at: string;
  finished_at?: string;
  answers: SessionAnswerResponse[];
  certificate_share_code?: string;
}

function toSessionError(err: unknown): SessionError {
  if (err instanceof ApiError) {
    return { code: err.code, message: err.message };
  }
  return { code: "network_error", message: "Tarmoq xatosi. Internetni tekshiring." };
}

function parseExplanationItems(raw: unknown): QuestionExplanationItem[] {
  if (!Array.isArray(raw)) return [];

  const items: QuestionExplanationItem[] = [];
  for (const item of raw) {
    if (!item || typeof item !== "object") continue;
    const candidate = item as { position?: unknown; correct?: unknown; text?: unknown };
    if (
      typeof candidate.position === "number" &&
      Number.isFinite(candidate.position) &&
      typeof candidate.correct === "boolean" &&
      typeof candidate.text === "string"
    ) {
      items.push({
        position: candidate.position,
        correct: candidate.correct,
        text: candidate.text,
      });
    }
  }
  return items;
}

/**
 * Normalize the real backend `{ type, text, items? }` schema while accepting
 * the old frontend-only `content` field as a compatibility fallback.
 */
function narrowExplanation(raw: RawExplanationPayload | null | undefined): QuestionExplanation | null {
  if (!raw || typeof raw !== "object" || !Array.isArray(raw.blocks)) return null;

  const blocks: QuestionExplanationBlock[] = [];
  for (const block of raw.blocks) {
    if (!block || typeof block !== "object") continue;
    const candidate = block as {
      type?: unknown;
      text?: unknown;
      content?: unknown;
      items?: unknown;
    };
    const text =
      typeof candidate.text === "string"
        ? candidate.text
        : typeof candidate.content === "string"
          ? candidate.content
          : undefined;
    const items = parseExplanationItems(candidate.items);
    if (text === undefined && items.length === 0) continue;

    const itemText = items.map((item) => item.text).join("\n");
    const content = [text, itemText].filter((part): part is string => Boolean(part)).join("\n");
    blocks.push({
      type: typeof candidate.type === "string" ? candidate.type : "",
      content,
      ...(text !== undefined ? { text } : {}),
      ...(items.length > 0 ? { items } : {}),
    });
  }

  return blocks.length > 0 ? { legal_refs: raw.legal_refs, blocks } : null;
}

function chooseDefined<T>(primary: T | null | undefined, fallback: T | null | undefined): T | undefined {
  return primary !== undefined && primary !== null
    ? primary
    : fallback !== undefined && fallback !== null
      ? fallback
      : undefined;
}

/** Build a question using only the authenticated session-scoped response. */
function toQuestionItem(
  detail: QuestionDetailResponse,
  persisted?: SessionAnswerResponse
): SessionQuestionItem {
  return {
    id: detail.id,
    question: detail.text,
    image_url: detail.image_url,
    answers: detail.answers.map((answer) => ({
      id: answer.id,
      position: answer.position,
      text: answer.text,
      image_url: answer.image_url,
    })),
    position: persisted?.position ?? detail.position,
    answered: persisted?.answered ?? detail.answered,
    user_answer_id: chooseDefined(persisted?.user_answer_id, detail.user_answer_id) ?? null,
    correct: chooseDefined(persisted?.correct, detail.correct),
    correct_answer_id: chooseDefined(persisted?.correct_answer_id, detail.correct_answer_id),
    explanation: narrowExplanation(detail.explanation),
  };
}

/** Promise.all preserves the canonical input order supplied by the session. */
async function fetchSessionQuestions(
  sessionId: string,
  orderedAnswers: SessionAnswerResponse[],
  locale: string
): Promise<SessionQuestionItem[]> {
  const details = await Promise.all(
    orderedAnswers.map((answer) =>
      apiGet<QuestionDetailResponse>(
        `sessions/${sessionId}/questions/${answer.question_id}?locale=${encodeURIComponent(locale)}`
      )
    )
  );
  return details.map((detail, index) => toQuestionItem(detail, orderedAnswers[index]));
}

function orderedQuestionRefs(questionIds: string[]): SessionAnswerResponse[] {
  return questionIds.map((questionId, index) => ({
    question_id: questionId,
    position: index + 1,
    answered: false,
  }));
}

function sortSessionAnswers(answers: SessionAnswerResponse[]): SessionAnswerResponse[] {
  return answers
    .map((answer, serverIndex) => ({ answer, serverIndex }))
    .sort((left, right) =>
      left.answer.position === right.answer.position
        ? left.serverIndex - right.serverIndex
        : left.answer.position - right.answer.position
    )
    .map(({ answer }) => answer);
}

/** Reconstruct the deadline from immutable server values; never restart it. */
function remainingSeconds(startedAt: string, timeLimitSec: number | null): number | null {
  if (timeLimitSec === null) return null;
  const startedAtMs = Date.parse(startedAt);
  if (!Number.isFinite(startedAtMs)) return null;
  const remainingMs = startedAtMs + timeLimitSec * 1000 - Date.now();
  return Math.max(0, Math.ceil(remainingMs / 1000));
}

async function fetchSessionState(sessionId: string, locale: string): Promise<SessionState> {
  const detail = await apiGet<SessionDetailResponse>(`sessions/${sessionId}`);
  const orderedAnswers = sortSessionAnswers(detail.answers);
  const questions = await fetchSessionQuestions(sessionId, orderedAnswers, locale);
  const timeLimitSec = detail.time_limit_sec ?? null;
  const completed = detail.status !== "in_progress";

  return {
    id: detail.id,
    mode: detail.mode,
    time_limit_sec: timeLimitSec,
    remaining_sec: remainingSeconds(detail.started_at, timeLimitSec),
    status: completed ? "completed" : "active",
    questions,
    score: detail.score ?? null,
    total: detail.total,
    stopped_reason: detail.stopped_reason || null,
    passed: completed ? detail.status === "passed" : null,
    completed_at: detail.finished_at ?? null,
    certificate_share_code: detail.certificate_share_code ?? null,
  };
}

export function useSessionEngine(_initialSessionId?: string) {
  const [session, setSession] = useState<SessionState | null>(null);
  const sessionRef = useRef<SessionState | null>(null);
  const localeRef = useRef<string>(defaultLocale);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<SessionError | null>(null);

  const commitSession = useCallback((next: SessionState | null) => {
    sessionRef.current = next;
    setSession(next);
  }, []);

  const startSession = useCallback(
    async (mode: SessionMode, options?: StartSessionOptions): Promise<SessionState | null> => {
      setLoading(true);
      setError(null);
      commitSession(null);
      const locale = options?.locale ?? defaultLocale;
      localeRef.current = locale;

      try {
        const payload: Record<string, unknown> = { mode, locale };
        if (options?.variant_id !== undefined && options.variant_id !== null) {
          payload.variant_id = String(options.variant_id);
        }
        if (options?.category_id !== undefined && options.category_id !== null) {
          payload.category_id = String(options.category_id);
        }
        if (options?.sign_id !== undefined && options.sign_id !== null) {
          payload.sign_id = String(options.sign_id);
        }
        if (options?.has_image !== undefined) {
          payload.has_image = options.has_image;
        }
        if (options?.question_count !== undefined && options.question_count !== null) {
          payload.count = options.question_count;
        }

        const created = await apiPost<StartSessionResponse>("sessions", payload);
        const questions = await fetchSessionQuestions(
          created.id,
          orderedQuestionRefs(created.question_ids),
          locale
        );
        const state: SessionState = {
          id: created.id,
          mode: created.mode,
          time_limit_sec: created.time_limit_sec,
          remaining_sec: remainingSeconds(created.started_at, created.time_limit_sec),
          status: "active",
          questions,
          score: null,
          total: created.total,
          stopped_reason: null,
          passed: null,
          completed_at: null,
        };
        commitSession(state);
        return state;
      } catch (err: unknown) {
        setError(toSessionError(err));
        return null;
      } finally {
        setLoading(false);
      }
    },
    [commitSession]
  );

  const loadSession = useCallback(
    async (sessionId: string, locale?: string): Promise<SessionState | null> => {
      setLoading(true);
      setError(null);
      commitSession(null);
      const resolvedLocale = locale ?? defaultLocale;
      localeRef.current = resolvedLocale;

      try {
        const state = await fetchSessionState(sessionId, resolvedLocale);
        commitSession(state);
        return state;
      } catch (err: unknown) {
        setError(toSessionError(err));
        return null;
      } finally {
        setLoading(false);
      }
    },
    [commitSession]
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
        const response = await apiPost<SubmitAnswerResponse>(`sessions/${sessionId}/answers`, {
          question_id: questionId,
          answer_id: answerId,
        });

        if (response.recorded) {
          const current = sessionRef.current;
          if (current) {
            commitSession({
              ...current,
              questions: current.questions.map((question) => {
                if (question.id !== questionId) return question;
                return {
                  ...question,
                  user_answer_id: answerId,
                  answered: true,
                  ...(response.correct !== undefined ? { correct: response.correct } : {}),
                  ...(response.correct_answer_id !== undefined
                    ? { correct_answer_id: response.correct_answer_id }
                    : {}),
                  ...(response.explanation !== undefined
                    ? { explanation: narrowExplanation(response.explanation) }
                    : {}),
                };
              }),
            });
          }
        }

        // A stopped exam is already finalized server-side. Reload its detail
        // and every scoped question so withheld feedback becomes available.
        if (response.stopped) {
          try {
            commitSession(await fetchSessionState(sessionId, localeRef.current));
          } catch (reloadError: unknown) {
            const current = sessionRef.current;
            if (current) {
              commitSession({
                ...current,
                // The backend has completed the exam, but its authoritative
                // score/pass result could not be reloaded. Keep a distinct
                // recoverable state instead of fabricating 0/failed.
                status: "result_pending",
                stopped_reason: response.stop_reason ?? current.stopped_reason,
              });
            }
            setError(toSessionError(reloadError));
          }
        }

        // Return the backend response itself, including explanation/stopped
        // fields; consumers never receive a client-generated grading result.
        return response;
      } catch (err: unknown) {
        setError(toSessionError(err));
        return null;
      } finally {
        setSubmitting(false);
      }
    },
    [commitSession]
  );

  const finishSession = useCallback(
    async (sessionId: string): Promise<SessionState | null> => {
      setLoading(true);
      setError(null);

      try {
        const response = await apiPost<FinishSessionResponse>(`sessions/${sessionId}/finish`);

        // Finishing changes exam disclosure rules, so the finish DTO alone is
        // insufficient: reload session answers and each scoped question.
        try {
          const reloaded = await fetchSessionState(sessionId, localeRef.current);
          commitSession(reloaded);
          return reloaded;
        } catch (reloadError: unknown) {
          const current = sessionRef.current;
          if (!current) {
            setError(toSessionError(reloadError));
            return null;
          }
          const completed: SessionState = {
            ...current,
            status: "completed",
            score: response.score,
            total: response.total,
            stopped_reason: response.stopped_reason || null,
            passed: response.status === "passed",
            certificate_share_code: response.certificate_share_code ?? current.certificate_share_code ?? null,
          };
          commitSession(completed);
          setError(toSessionError(reloadError));
          return completed;
        }
      } catch (err: unknown) {
        setError(toSessionError(err));
        return null;
      } finally {
        setLoading(false);
      }
    },
    [commitSession]
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
