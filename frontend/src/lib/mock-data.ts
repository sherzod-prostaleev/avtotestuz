import { OFFICIAL_TICKET_COUNT } from "@/lib/content-counts";

export interface MockAnswer {
  id: string;
  shortcutLabel: string;
  text: string;
}

export interface MockQuestion {
  id: string;
  text: string;
  hasImage: boolean;
  answers: MockAnswer[];
  correctAnswerId: string;
}

export const demoQuestion: MockQuestion = {
  id: "demo-1",
  text: "Ushbu chorrahada svetofor ishlamayapti. Kim birinchi bo'lib o'tadi?",
  hasImage: true,
  answers: [
    { id: "a1", shortcutLabel: "F1", text: "Chapdan keluvchi haydovchi" },
    { id: "a2", shortcutLabel: "F2", text: "O'ngdan keluvchi haydovchi" },
    { id: "a3", shortcutLabel: "F3", text: "To'g'ri harakatlanuvchi haydovchi" },
    { id: "a4", shortcutLabel: "F4", text: "Kim tezroq yetib borsa" },
  ],
  correctAnswerId: "a2",
};

export const mockExamQuestions: MockQuestion[] = [
  demoQuestion,
  {
    id: "q2",
    text: "\"To'xtash taqiqlangan\" belgisi qaysi masofagacha amal qiladi?",
    hasImage: true,
    answers: [
      { id: "b1", shortcutLabel: "F1", text: "Keyingi chorrahagacha" },
      { id: "b2", shortcutLabel: "F2", text: "100 metrgacha" },
      { id: "b3", shortcutLabel: "F3", text: "Belgi o'rnatilgan hudud oxirigacha" },
    ],
    correctAnswerId: "b3",
  },
  {
    id: "q3",
    text: "Tormoz masofasi qanday omillarga bog'liq?",
    hasImage: false,
    answers: [
      { id: "c1", shortcutLabel: "F1", text: "Faqat tezlikka" },
      { id: "c2", shortcutLabel: "F2", text: "Tezlik, yo'l sirti va shina holatiga" },
      { id: "c3", shortcutLabel: "F3", text: "Faqat haydovchi tajribasiga" },
    ],
    correctAnswerId: "c2",
  },
  {
    id: "q4",
    text: "Imtihon rejimida noto'g'ri javob berilganda ekranda nima ko'rsatiladi?",
    hasImage: false,
    answers: [
      { id: "d1", shortcutLabel: "F1", text: "Darhol to'g'ri javob ko'rsatiladi" },
      { id: "d2", shortcutLabel: "F2", text: "Faqat \"javob qabul qilindi\" belgisi" },
      { id: "d3", shortcutLabel: "F3", text: "Ovozli ogohlantirish" },
    ],
    correctAnswerId: "d2",
  },
];

export const mockProfile = {
  name: "Aziz",
  isVip: false,
  streak: { current: 12, best: 21, todayDone: 6, dailyGoal: 10 },
  readinessPercent: 68,
};

// Category names match the 13 real, approved taxonomy codes
// (backend/cmd/convertavtoimtihon/categories.go) — not invented labels.
export const mockCategoryMastery = [
  { categoryName: "Yo'l belgilari va chizig'i", masteryPercent: 74 },
  { categoryName: "Chorrahalar va yo'l ustunligi", masteryPercent: 46 },
  { categoryName: "YHH, tez tibbiy yordam va tormozlash", masteryPercent: 28 },
  { categoryName: "To'xtash va to'xtab turish", masteryPercent: 83 },
];

export const proofStats = [
  { value: "1235", label: "savol" },
  { value: String(OFFICIAL_TICKET_COUNT), label: "bilet" },
  { value: "13", label: "mavzu" },
  { value: "3", label: "til" },
];
