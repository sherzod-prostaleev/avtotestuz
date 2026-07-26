import { parseLegalBody } from "@/lib/site-legal";

/** Renders CMS legal plain text (## headings + blank-line paragraphs). */
export function LegalCmsBody({ body }: { body: string }) {
  const blocks = parseLegalBody(body);
  if (blocks.length === 0) return null;

  return (
    <div className="space-y-6">
      {blocks.map((block, index) =>
        block.type === "h2" ? (
          <h2
            key={`h-${index}-${block.text}`}
            className="font-display text-lg font-bold text-foreground"
          >
            {block.text}
          </h2>
        ) : (
          <p
            key={`p-${index}`}
            className="whitespace-pre-wrap text-muted-foreground"
          >
            {block.text}
          </p>
        )
      )}
    </div>
  );
}
