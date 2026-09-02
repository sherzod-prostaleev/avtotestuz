"use client";

import { useRef, useState } from "react";
import { Paperclip, Send } from "lucide-react";

type Props = {
  disabled?: boolean;
  placeholder: string;
  sendLabel: string;
  attachLabel: string;
  onSend: (text: string, file: File | null) => Promise<void>;
};

export function ChatComposer({
  disabled,
  placeholder,
  sendLabel,
  attachLabel,
  onSend,
}: Props) {
  const [text, setText] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  async function submit() {
    const body = text.trim();
    if ((!body && !file) || busy || disabled) return;
    setBusy(true);
    try {
      await onSend(body, file);
      setText("");
      setFile(null);
      if (inputRef.current) inputRef.current.value = "";
    } finally {
      setBusy(false);
    }
  }

  return (
    // Phone: the artboard has no bar under the thread — the three controls sit
    // straight on the page, 12px in from the edges and 12px above the tab bar.
    <div className="border-t border-border bg-card/80 p-2 backdrop-blur max-md:border-t-0 max-md:bg-transparent max-md:p-0 max-md:px-3 max-md:pb-3 max-md:backdrop-blur-none">
      {file ? (
        <div className="mb-2 flex items-center justify-between gap-2 rounded-lg bg-muted px-2 py-1 text-xs">
          <span className="truncate">{file.name}</span>
          <button type="button" className="text-muted-foreground underline" onClick={() => setFile(null)}>
            ×
          </button>
        </div>
      ) : null}
      <div className="flex items-end gap-2">
        <label className="flex h-10 w-10 shrink-0 cursor-pointer items-center justify-center rounded-xl border border-border bg-background text-muted-foreground hover:bg-muted max-md:h-11 max-md:w-11 max-md:bg-card">
          <Paperclip className="h-4 w-4 max-md:h-5 max-md:w-5" aria-hidden />
          <span className="sr-only">{attachLabel}</span>
          <input
            ref={inputRef}
            type="file"
            className="hidden"
            accept="image/*,.pdf,.txt,.doc,.docx,.xls,.xlsx,.zip"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </label>
        <textarea
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              void submit();
            }
          }}
          rows={1}
          placeholder={placeholder}
          disabled={busy || disabled}
          className="min-h-10 max-h-32 flex-1 resize-none rounded-xl border border-border bg-background px-3 py-2 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring max-md:min-h-11 max-md:bg-card max-md:text-[15px]"
        />
        <button
          type="button"
          aria-label={sendLabel}
          disabled={busy || disabled || (!text.trim() && !file)}
          onClick={() => void submit()}
          className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-accent text-accent-foreground disabled:opacity-40 max-md:h-11 max-md:w-11 max-md:shadow-3d"
        >
          <Send className="h-4 w-4 max-md:h-5 max-md:w-5" aria-hidden />
        </button>
      </div>
    </div>
  );
}
