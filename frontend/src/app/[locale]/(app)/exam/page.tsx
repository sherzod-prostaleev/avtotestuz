// Exam variety chooser: /[locale]/exam.
//
// Every "Imtihon" entry point in the learner app lands here instead of
// starting a session outright, because there are now two official exams
// (20-question standard and 50-question restore) and the runner cannot guess
// which one the learner is preparing for.
import { ExamModePicker } from "@/components/exam/exam-mode-picker";

export default function ExamPage() {
  return <ExamModePicker />;
}
