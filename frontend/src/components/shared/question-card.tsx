"use client";

import { ZoomIn, Lightbulb } from "lucide-react";
import { QuestionExplanation } from "@/hooks/use-session-engine";

interface QuestionCardProps {
  questionNumber: number;
  totalQuestions: number;
  questionText?: string;
  text?: string;
  imageUrl?: string | null;
  hasImage?: boolean;
  onImageClick?: () => void;
  explanation?: QuestionExplanation | null;
}

export function QuestionCard({
  questionNumber,
  totalQuestions,
  questionText,
  text,
  imageUrl,
  hasImage,
  onImageClick,
  explanation,
}: QuestionCardProps) {
  const displayText = questionText || text || "";
  const finalImageUrl = imageUrl || (hasImage ? "/placeholder.png" : null);

  return (
    <div className="space-y-4">
      {/* Question Header & Badge */}
      <div className="flex items-center justify-between">
        <span className="inline-flex items-center gap-1.5 rounded-full border border-accent/30 bg-accent/10 px-3.5 py-1 text-xs font-extrabold text-accent">
          Savol {questionNumber} / {totalQuestions}
        </span>
      </div>

      {/* Question Text */}
      <h2 className="font-display text-lg font-bold leading-snug tracking-tight text-foreground sm:text-xl">
        {displayText}
      </h2>

      {/* Image Block (Zoomable) */}
      {finalImageUrl && (
        <div
          onClick={onImageClick}
          className="group relative cursor-pointer overflow-hidden rounded-2xl border border-border bg-black/5 transition-all hover:border-accent/60 hover:shadow-lg"
        >
          {hasImage && !imageUrl ? (
            <div className="flex h-36 w-full items-center justify-center text-xs font-bold text-muted-foreground">
              Savol rasmi
            </div>
          ) : (
            <img
              src={finalImageUrl}
              alt={`Savol ${questionNumber} rasmi`}
              className="mx-auto max-h-72 w-full object-contain transition-transform duration-300 group-hover:scale-[1.02]"
            />
          )}
          <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
            <span className="flex items-center gap-1.5 rounded-full bg-white/90 px-3.5 py-1.5 text-xs font-bold text-slate-900 shadow-md backdrop-blur-sm">
              <ZoomIn className="h-4 w-4" /> Kattalashtirish
            </span>
          </div>
        </div>
      )}

      {/* Explanation Block */}
      {explanation && explanation.blocks && explanation.blocks.length > 0 && (
        <div className="mt-6 rounded-2xl border border-accent/30 bg-accent/5 p-4 space-y-2">
          <div className="flex items-center gap-2 text-xs font-bold uppercase tracking-wider text-accent">
            <Lightbulb className="h-4 w-4 text-gold" />
            <span>Rasmiy YHQ Tushuntirishi</span>
          </div>
          {explanation.blocks.map((block, idx) => (
            <div key={idx} className="text-xs leading-relaxed text-foreground/90">
              {block.content}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
