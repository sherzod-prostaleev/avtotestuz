import { ImageIcon } from "lucide-react";
import { Card } from "@/components/ui/card";

export interface QuestionCardProps {
  questionNumber: number;
  totalQuestions: number;
  text: string;
  hasImage?: boolean;
}

export function QuestionCard({ questionNumber, totalQuestions, text, hasImage = false }: QuestionCardProps) {
  return (
    <Card className="p-6 md:p-8">
      <p className="mb-4 text-sm font-semibold text-muted-foreground">
        Savol {questionNumber} / {totalQuestions}
      </p>
      <p className="font-display text-xl font-bold leading-snug md:text-2xl">{text}</p>
      {hasImage && (
        <div className="mt-6 flex h-48 items-center justify-center rounded-md border border-dashed border-border bg-background/40 text-muted-foreground">
          <ImageIcon aria-hidden className="mr-2 h-6 w-6" />
          <span className="text-sm">Savol rasmi</span>
        </div>
      )}
    </Card>
  );
}
