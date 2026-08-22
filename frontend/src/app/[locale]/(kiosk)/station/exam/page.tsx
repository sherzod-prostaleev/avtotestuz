// Kiosk exam chooser: /[locale]/station/exam.
//
// This is a distinct URL from the learner app's /[locale]/exam specifically so
// the middleware exemption in src/proxy.ts stays narrow (only "station" and
// everything under it is login-free) instead of unauthenticating /exam for
// every learner on the platform. It reuses the same component in kiosk mode,
// which keeps every destination under /station/... and never offers the VIP
// paywall — see ExamModePickerProps in the imported module.
import { ExamModePicker } from "@/components/exam/exam-mode-picker";

export default function KioskExamPage() {
  return <ExamModePicker kiosk />;
}
