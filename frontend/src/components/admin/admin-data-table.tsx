"use client";

import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type Cell,
  type ColumnDef,
  type Row as TanRow,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useRef } from "react";
import { useTranslations } from "next-intl";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";
import { useIsCompact } from "@/hooks/use-media-query";

export type AdminColumnMeta = {
  /** Hide this column in the mobile card (table view only). */
  hideOnCard?: boolean;
  /** Render as the card's title line instead of a labelled row. */
  cardTitle?: boolean;
  /** Right-align the desktop cell (numbers). */
  numeric?: boolean;
};

function metaOf<T>(cell: Cell<T, unknown>): AdminColumnMeta {
  return (cell.column.columnDef.meta ?? {}) as AdminColumnMeta;
}

export type AdminDataTableProps<T> = {
  data: T[];
  columns: ColumnDef<T, unknown>[];
  emptyTitle: string;
  emptyDescription?: string;
  maxHeight?: number;
  estimateRowHeight?: number;
  getRowId?: (row: T) => string;
  /** Optional bespoke mobile card. When omitted, cards are derived from `columns`. */
  renderCard?: (row: T) => React.ReactNode;
  /** Force a representation; default is viewport-driven. */
  variant?: "auto" | "table" | "cards";
};

/**
 * One table primitive for every admin directory.
 *
 * Desktop renders a virtualized table whose rows are *measured*, not assumed —
 * a wrapping cell grows its own row instead of overlapping the next one.
 * Below `md` the same `columns` are re-projected as a card list, so a page gets
 * a correct phone layout without writing a second markup tree.
 */
export function AdminDataTable<T>({
  data,
  columns,
  emptyTitle,
  emptyDescription,
  maxHeight = 560,
  estimateRowHeight = 48,
  getRowId,
  renderCard,
  variant = "auto",
}: AdminDataTableProps<T>) {
  const t = useTranslations("AdminTable");
  const isCompact = useIsCompact();
  const parentRef = useRef<HTMLDivElement>(null);

  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getRowId: getRowId ? (row) => getRowId(row) : undefined,
  });
  const rows = table.getRowModel().rows;

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => estimateRowHeight,
    overscan: 12,
    // Measure the real rendered height so wrapping content cannot overlap.
    measureElement: (el) => el.getBoundingClientRect().height,
  });

  if (data.length === 0) {
    return <AdminEmptyState title={emptyTitle} description={emptyDescription} />;
  }

  const asCards = variant === "cards" || (variant === "auto" && isCompact);

  // Virtual rows stay in normal table flow and the scrolled-past space is held
  // open by two spacer rows. The obvious alternative — absolutely positioning
  // each <tr> — silently breaks the table: an absolutely positioned element is
  // blockified, so `display: table-row` computes to `block`, the row stops
  // participating in the table's column-width algorithm, and every row sizes
  // its own columns independently of the sticky header above it.
  const virtualRows = virtualizer.getVirtualItems();
  const columnCount = table.getAllLeafColumns().length;
  const padTop = virtualRows.length ? virtualRows[0].start : 0;
  const padBottom = virtualRows.length
    ? virtualizer.getTotalSize() - virtualRows[virtualRows.length - 1].end
    : 0;

  if (asCards) {
    return (
      <ul role="list" aria-label={t("cardsLabel")} className="space-y-2.5">
        {rows.map((row) => (
          <li key={row.id} className="admin-panel p-3.5">
            {renderCard ? renderCard(row.original) : <DerivedCard row={row} />}
          </li>
        ))}
      </ul>
    );
  }

  return (
    <div className="admin-panel overflow-hidden">
      <div
        ref={parentRef}
        className="admin-scroll-x relative w-full overflow-y-auto focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
        style={{ maxHeight }}
        role="region"
        aria-label={t("rowsLabel")}
        tabIndex={0}
      >
        <table className="w-full border-collapse text-left text-sm">
          <thead className="sticky top-0 z-10 bg-card">
            {table.getHeaderGroups().map((hg) => (
              <tr key={hg.id} className="border-b border-border">
                {hg.headers.map((h) => {
                  const meta = (h.column.columnDef.meta ?? {}) as AdminColumnMeta;
                  return (
                    <th
                      key={h.id}
                      scope="col"
                      className={`admin-label whitespace-nowrap px-3 py-2.5 ${
                        meta.numeric ? "text-right" : "text-left"
                      }`}
                    >
                      {h.isPlaceholder
                        ? null
                        : flexRender(h.column.columnDef.header, h.getContext())}
                    </th>
                  );
                })}
              </tr>
            ))}
          </thead>
          <tbody>
            {padTop > 0 ? (
              <tr aria-hidden>
                <td colSpan={columnCount} style={{ height: padTop, padding: 0, border: 0 }} />
              </tr>
            ) : null}
            {virtualRows.map((vRow) => {
              const row = rows[vRow.index];
              return (
                <tr
                  key={row.id}
                  data-index={vRow.index}
                  ref={virtualizer.measureElement}
                  className="border-b border-border/60 hover:bg-accent/[0.06]"
                >
                  {row.getVisibleCells().map((cell) => {
                    const meta = metaOf(cell);
                    return (
                      <td
                        key={cell.id}
                        className={`px-3 py-2.5 align-middle ${
                          meta.numeric ? "text-right tabular-nums" : ""
                        }`}
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    );
                  })}
                </tr>
              );
            })}
            {padBottom > 0 ? (
              <tr aria-hidden>
                <td colSpan={columnCount} style={{ height: padBottom, padding: 0, border: 0 }} />
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/** Card projection of a row: title column on top, remaining columns as label/value pairs. */
function DerivedCard<T>({ row }: { row: TanRow<T> }) {
  const cells = row.getVisibleCells();
  const titleCell = cells.find((c) => metaOf(c).cardTitle);
  const bodyCells = cells.filter((c) => {
    const meta = metaOf(c);
    return !meta.cardTitle && !meta.hideOnCard;
  });

  return (
    <div className="space-y-2">
      {titleCell ? (
        <div className="font-display text-sm font-bold">
          {flexRender(titleCell.column.columnDef.cell, titleCell.getContext())}
        </div>
      ) : null}
      <dl className="grid grid-cols-[minmax(6rem,auto)_1fr] gap-x-3 gap-y-1.5">
        {bodyCells.map((cell) => {
          const meta = metaOf(cell);
          const rawHeader = cell.column.columnDef.header;
          const label = typeof rawHeader === "string" ? rawHeader.trim() : "";
          return (
            // `admin-card-row` hides itself when its <dd> renders nothing. Cells
            // routinely render null — a row action wrapped in a PermissionGate,
            // or an action that only exists in one status — and without this a
            // read-only operator gets a card full of labels with no values.
            <div key={cell.id} className="admin-card-row contents">
              {label ? (
                <dt className="admin-label self-center">{label}</dt>
              ) : (
                // A blank header (a selection checkbox column) must not leave an
                // empty label sitting in the grid's first column. The cell's own
                // control carries the accessible name — announcing the internal
                // column id here would read an English identifier aloud in an
                // Uzbek UI.
                <dt className="sr-only" aria-hidden />
              )}
              {/* A money column stays a money column on a phone: lining figures
                  so amounts of the same magnitude line up when cards stack. */}
              <dd
                className={`min-w-0 break-words text-sm ${label ? "" : "col-span-2"} ${
                  meta.numeric ? "tabular-nums" : ""
                }`}
              >
                {flexRender(cell.column.columnDef.cell, cell.getContext())}
              </dd>
            </div>
          );
        })}
      </dl>
    </div>
  );
}
