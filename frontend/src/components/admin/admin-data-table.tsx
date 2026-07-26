"use client";

import {
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
} from "@tanstack/react-table";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useRef } from "react";
import { AdminEmptyState } from "@/components/admin/admin-empty-state";

type AdminDataTableProps<T> = {
  data: T[];
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  columns: ColumnDef<T, any>[];
  emptyTitle: string;
  emptyDescription?: string;
  maxHeight?: number;
  estimateRowHeight?: number;
  getRowId?: (row: T) => string;
};

/** Virtualized table for large admin directories. */
export function AdminDataTable<T>({
  data,
  columns,
  emptyTitle,
  emptyDescription,
  maxHeight = 520,
  estimateRowHeight = 44,
  getRowId,
}: AdminDataTableProps<T>) {
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
  });

  if (data.length === 0) {
    return <AdminEmptyState title={emptyTitle} description={emptyDescription} />;
  }

  return (
    <div className="overflow-hidden rounded-2xl border border-border/80 bg-card/40">
      <div className="overflow-x-auto">
        <div
          ref={parentRef}
          className="relative w-full min-w-[720px] overflow-auto"
          style={{ maxHeight }}
          role="region"
          aria-label="table"
        >
          <table className="w-full border-collapse text-left text-sm">
            <thead className="sticky top-0 z-10 bg-[hsl(220_28%_8%)] text-[11px] uppercase tracking-wider text-muted-foreground">
              {table.getHeaderGroups().map((hg) => (
                <tr key={hg.id} className="border-b border-border">
                  {hg.headers.map((h) => (
                    <th key={h.id} className="px-3 py-2.5 font-bold">
                      {h.isPlaceholder
                        ? null
                        : flexRender(h.column.columnDef.header, h.getContext())}
                    </th>
                  ))}
                </tr>
              ))}
            </thead>
            <tbody
              style={{
                height: `${virtualizer.getTotalSize()}px`,
                position: "relative",
              }}
            >
              {virtualizer.getVirtualItems().map((vRow) => {
                const row = rows[vRow.index];
                return (
                  <tr
                    key={row.id}
                    className="absolute left-0 w-full border-b border-border/40 hover:bg-accent/[0.05]"
                    style={{
                      height: `${vRow.size}px`,
                      transform: `translateY(${vRow.start}px)`,
                    }}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <td key={cell.id} className="px-3 py-2 align-middle">
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </td>
                    ))}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
