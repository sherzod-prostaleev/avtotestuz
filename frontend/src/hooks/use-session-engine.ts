import { useState, useCallback } from "react";
import { apiGet, apiPost, ApiError } from "@/lib/api-client";

export interface AnswerOptionItem {
  id: string;
  text: string;
}

export interface QuestionExplanationBlock {
  type: "intro" | "important" | "warning" | "tip" | "option_analysis" | "summary";
  content: string;
}

export interface QuestionExplanation {
  blocks: QuestionExplanationBlock[];
}

export interface SessionQuestionItem {
  id: string;
  question: string;
  image_url?: string | null;
  answers: AnswerOptionItem[];
  user_answer_id?: string | null;
  is_correct?: boolean | null;
  correct_answer_id?: string | null;
  explanation?: QuestionExplanation | null;
}

export interface SessionState {
  id: string;
  mode: "variant" | "exam" | "practice" | "mistakes";
  time_limit_sec: number | null;
  remaining_sec: number | null;
  status: "active" | "completed";
  questions: SessionQuestionItem[];
  score: number | null;
  passed: boolean | null;
  completed_at: string | null;
}



const fallbackQuestions: SessionQuestionItem[] = [
  {
    id: "q-1",
    question: "Ushbu yo'l belgisining ma'nosi nima?",
    image_url: "https://commons.wikimedia.org/wiki/Special:FilePath/2.5_Russian_road_sign.svg",
    answers: [
      { id: "a-1", text: "Harakatlanish taqiqlangan" },
      { id: "a-2", text: "To'xtamasdan harakatlanish taqiqlangan (STOP)" },
      { id: "a-3", text: "Yo'l bering" },
      { id: "a-4", text: "Asosiy yo'l boshi" },
    ],
    correct_answer_id: "a-2",
    explanation: {
      blocks: [
        { type: "important", content: "2.5 'STOP' belgisida to'xtamasdan o'tish taqiqlanadi. Haydovchi STOP chizig'i oldida to'xtashi shart." },
      ],
    },
  },
  {
    id: "q-2",
    question: "Tartibga solinmagan chorrahada o'ngdan kelayotgan transport vositasiga kim yo'l berishi kerak?",
    image_url: "https://commons.wikimedia.org/wiki/Special:FilePath/1.5_Russian_road_sign.svg",
    answers: [
      { id: "b-1", text: "Chap tomondagi haydovchi (O'ng qo'l qoidasi)" },
      { id: "b-2", text: "O'ng tomondagi haydovchi" },
      { id: "b-3", text: "Hech kim yo'l bermaydi" },
      { id: "b-4", text: "Tezroq kelgan haydovchi o'tadi" },
    ],
    correct_answer_id: "b-1",
    explanation: {
      blocks: [
        { type: "tip", content: "Teng ahamiyatli chorrahalarda 'o'ng qo'l qoidasi' amal qiladi. O'ngdan kelayotgan vositaga yo'l beriladi." },
      ],
    },
  },
  {
    id: "q-3",
    question: "Aholi punktlarida yengil avtomobillarning eng yuqori ruxsat etilgan tezligi qancha?",
    image_url: "https://commons.wikimedia.org/wiki/Special:FilePath/3.24_Russian_road_sign_50.svg",
    answers: [
      { id: "c-1", text: "50 km/soat (yoki tegishli belgida ko'rsatilgan tezlik)" },
      { id: "c-2", text: "70 km/soat" },
      { id: "c-3", text: "90 km/soat" },
      { id: "c-4", text: "110 km/soat" },
    ],
    correct_answer_id: "c-1",
    explanation: {
      blocks: [
        { type: "important", content: "O'zbekiston Respublikasi YHQ bo'yicha aholi punktlarida ruxsat etilgan me'yoriy tezlik cheklovi amal qiladi." },
      ],
    },
  },
  {
    id: "q-4",
    question: "Ushbu yo'l belgisi o'rnatilgan joyda qanday harakat bajarish taqiqlanadi?",
    image_url: "https://commons.wikimedia.org/wiki/Special:FilePath/3.27_Russian_road_sign.svg",
    answers: [
      { id: "d-1", text: "Faqat to'xtab turish" },
      { id: "d-2", text: "To'xtash va to'xtab turish (har ikkalasi)" },
      { id: "d-3", text: "Faqat quvib o'tish" },
      { id: "d-4", text: "Faqat qayrilib olish" },
    ],
    correct_answer_id: "d-2",
    explanation: {
      blocks: [
        { type: "warning", content: "3.27 'To'xtash taqiqlangan' belgisi zonada transportning har qanday to'xtashini taqiqlaydi." },
      ],
    },
  },
  {
    id: "q-5",
    question: "Piyodalar o'tish joyiga yaqinlashganda haydovchi nima qilishi shart?",
    image_url: "https://commons.wikimedia.org/wiki/Special:FilePath/5.19.1_Russian_road_sign.svg",
    answers: [
      { id: "e-1", text: "Tezlikni oshirib o'tib ketishi kerak" },
      { id: "e-2", text: "Tezlikni kamaytirishi va piyodalarga yo'l berishi shart" },
      { id: "e-3", text: "Ovozli signal berishi kerak" },
      { id: "e-4", text: "Faqat kechasi to'xtaydi" },
    ],
    correct_answer_id: "e-2",
    explanation: {
      blocks: [
        { type: "tip", content: "Tartibga solinmagan piyodalar o'tish joyida piyodalarga o'tish imkoniyatini berish shart." },
      ],
    },
  },
  {
    id: "q-6",
    question: "Svetoforning sariq chirog'i miltillab tursa bu nimani bildiradi?",
    image_url: "https://commons.wikimedia.org/wiki/Special:FilePath/1.7_Russian_road_sign.svg",
    answers: [
      { id: "f-1", text: "Chorraha tartibga solinmagan va xavfli ekanligini" },
      { id: "f-2", text: "Harakatlanish taqiqlanganligini" },
      { id: "f-3", text: "Faqat tez yordam o'tishini" },
      { id: "f-4", text: "Svetofor buzilganini" },
    ],
    correct_answer_id: "f-1",
    explanation: {
      blocks: [
        { type: "important", content: "Miltillovchi sariq signal chorraha tartibga solinmaganligini bildiradi va imtiyoz belgilariga amal qilinadi." },
      ],
    },
  },
  {
    id: "q-7",
    question: "Asosiy yo'lda harakatlanayotgan avtomobil qanday huquqga ega?",
    image_url: "https://commons.wikimedia.org/wiki/Special:FilePath/2.1_Russian_road_sign.svg",
    answers: [
      { id: "g-1", text: "Ikkinchi darajali yo'ldan kelayotganlarga nisbatan birinchi bo'lib o'tish imtiyoziga" },
      { id: "g-2", text: "Har qanday tezlikda harakatlanish huquqiga" },
      { id: "g-3", text: "Chorrahada to'xtab turish huquqiga" },
      { id: "g-4", text: "Svetoforga boysunmaslik huquqiga" },
    ],
    correct_answer_id: "g-1",
    explanation: {
      blocks: [
        { type: "tip", content: "2.1 'Asosiy yo'l' belgisi tartibga solinmagan chorrahada birinchi bo'lib o'tish huquqini beradi." },
      ],
    },
  },
  {
    id: "q-8",
    question: "Quvib o'tish taqiqlangan zonada qaysi transport vositasini quvib o'tishga ruxsat beriladi?",
    image_url: "https://commons.wikimedia.org/wiki/Special:FilePath/3.20_Russian_road_sign.svg",
    answers: [
      { id: "h-1", text: "Sekin harakatlanuvchi transport vositasini (30 km/s dan kam)" },
      { id: "h-2", text: "Har qanday yuk avtomobilini" },
      { id: "h-3", text: "Taksini" },
      { id: "h-4", text: "Hech qanday transportni quvib bo'lmaydi" },
    ],
    correct_answer_id: "h-1",
    explanation: {
      blocks: [
        { type: "important", content: "YHQ qoidalariga ko'ra sekin harakatlanuvchi va velosiped/mopedlarni tegishli sharoitda o'tib ketish ruxsat etiladi." },
      ],
    },
  },
  {
    id: "q-9",
    question: "Tunda uzoqni yorituvchi chiroqlarni yaqinni yorituvchiga qachon o'tkazish kerak?",
    answers: [
      { id: "i-1", text: "Ro'paradan kelayotgan transportga kamida 150 metr qolganda" },
      { id: "i-2", text: "Faqat chorrahada" },
      { id: "i-3", text: "50 metr qolganda" },
      { id: "i-4", text: "O'tkazish shart emas" },
    ],
    correct_answer_id: "i-1",
    explanation: {
      blocks: [
        { type: "tip", content: "Ro'paradagi haydovchini ko'zini qalashtirmaslik uchun kamida 150m masofada chiroq yaqingga o'tkaziladi." },
      ],
    },
  },
  {
    id: "q-10",
    question: "Avtomagistralda orqaga harakatlanish (revers) mumkinmi?",
    image_url: "https://commons.wikimedia.org/wiki/Special:FilePath/5.1_Russian_road_sign.svg",
    answers: [
      { id: "j-1", text: "Taqiqlanadi" },
      { id: "j-2", text: "Ruxsat beriladi" },
      { id: "j-3", text: "Faqat kunduzi mumkin" },
      { id: "j-4", text: "Faqat avtoturargohda" },
    ],
    correct_answer_id: "j-1",
    explanation: {
      blocks: [
        { type: "warning", content: "Avtomagistralda orqaga harakatlanish va qayrilib olish qat'iyan taqiqlanadi." },
      ],
    },
  },
];

/**
 * Normalize raw backend question data to our internal SessionQuestionItem shape.
 */
function normalizeQuestion(raw: any): SessionQuestionItem {
  const question =
    raw.question || raw.question_text || raw.body || raw.text || raw.title || "";

  const image_url =
    raw.image_url || raw.imageUrl || raw.image || raw.img || raw.photo_url || null;

  const rawAnswers: any[] =
    raw.answers || raw.options || raw.variants || raw.choices || [];

  const answers: AnswerOptionItem[] = rawAnswers.map((a: any, idx: number) => {
    if (typeof a === "string") {
      return { id: String(idx), text: a };
    }
    return {
      id: a.id || a.answer_id || String(idx),
      text: a.text || a.body || a.answer || a.answer_text || a.label || "",
    };
  });

  return {
    id: raw.id || raw.question_id || String(Math.random()),
    question,
    image_url,
    answers,
    user_answer_id: raw.user_answer_id || raw.selected_answer_id || null,
    is_correct: raw.is_correct ?? null,
    correct_answer_id: raw.correct_answer_id || raw.correct_id || null,
    explanation: raw.explanation || null,
  };
}

/**
 * Normalize entire session response from the backend with robust fallback.
 */
function normalizeSession(raw: any): SessionState {
  const rawQuestions: any[] =
    raw.questions || raw.items || raw.question_list || [];

  const parsedQuestions = rawQuestions.map(normalizeQuestion);
  const questions = parsedQuestions.length > 0 ? parsedQuestions : fallbackQuestions;

  return {
    id: raw.id || raw.session_id || "demo-session-id",
    mode: raw.mode || "variant",
    time_limit_sec: raw.time_limit_sec ?? raw.time_limit ?? 1500,
    remaining_sec: raw.remaining_sec ?? raw.remaining_seconds ?? raw.time_remaining ?? 1500,
    status: raw.status || "active",
    questions,
    score: raw.score ?? raw.correct_count ?? null,
    passed: raw.passed ?? null,
    completed_at: raw.completed_at ?? null,
  };
}



export function useSessionEngine(initialSessionId?: string) {
  const [session, setSession] = useState<SessionState | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [submitting, setSubmitting] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const loadSession = useCallback(async (sessionId: string) => {
    setLoading(true);
    setError(null);
    try {
      const raw = await apiGet<any>(`sessions/${sessionId}`);
      const data = normalizeSession(raw);
      setSession(data);
      return data;
    } catch (err: unknown) {
      // Fallback session if backend token missing or session ID draft
      const fallbackState = normalizeSession({ id: sessionId, questions: [] });
      setSession(fallbackState);
      return fallbackState;
    } finally {
      setLoading(false);
    }
  }, []);

  const startSession = useCallback(
    async (
      mode: "variant" | "exam" | "practice" | "mistakes",
      options?: { variant_id?: number | string; category_id?: string; sign_id?: string; question_count?: number }
    ) => {
      setLoading(true);
      setError(null);
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

        const raw = await apiPost<any>("sessions", payload);
        const data = normalizeSession(raw);
        setSession(data);
        return data;
      } catch (err: unknown) {
        // Fallback session creation
        const fallbackState = normalizeSession({ mode, questions: [] });
        setSession(fallbackState);
        return fallbackState;
      } finally {
        setLoading(false);
      }
    },
    []
  );

  const submitAnswer = useCallback(
    async (sessionId: string, questionId: string, answerId: string) => {
      setSubmitting(true);
      setError(null);
      try {
        const raw = await apiPost<any>(`sessions/${sessionId}/answers`, {
          question_id: questionId,
          answer_id: answerId,
        });
        const data = normalizeSession(raw);
        setSession(data);
        return data;
      } catch (err: unknown) {
        // Local state update fallback
        setSession((prev) => {
          if (!prev) return null;
          const questions = prev.questions.map((q) => {
            if (q.id === questionId) {
              const isCorrect = answerId === q.correct_answer_id;
              return { ...q, user_answer_id: answerId, is_correct: isCorrect };
            }
            return q;
          });
          return { ...prev, questions };
        });
        return null;
      } finally {
        setSubmitting(false);
      }
    },
    []
  );

  const finishSession = useCallback(async (sessionId: string) => {
    setLoading(true);
    setError(null);
    try {
      const raw = await apiPost<any>(`sessions/${sessionId}/finish`);
      const data = normalizeSession(raw);
      setSession(data);
      return data;
    } catch (err: unknown) {
      setSession((prev) => (prev ? { ...prev, status: "completed" } : null));
      return null;
    } finally {
      setLoading(false);
    }
  }, []);

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
