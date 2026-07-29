import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NextIntlClientProvider } from "next-intl";
import { type ColumnDef } from "@tanstack/react-table";
import { AdminDataTable } from "./admin-data-table";
import messages from "../../../messages/uz-Latn.json";

/**
 * Row actions an operator performs under time pressure — revoking a leaked
 * session, approving a payout — are exactly the ones they are most likely to
 * be doing from a phone. `AdminDataTable` projects a table into cards below
 * `md`, and a column tagged `hideOnCard` disappears from that projection.
 *
 * These tests pin the invariant that such a column reaches the card AND that
 * its handler fires with that row's own identifier, so a later "let's tidy up
 * the card" change cannot quietly take incident response away from phones
 * while leaving the suite green.
 */

type SessionRow = {
  id: string;
  active: boolean;
};

const rows: SessionRow[] = [
  { id: "sess-aaaa", active: true },
  { id: "sess-bbbb", active: true },
  { id: "sess-cccc", active: false },
];

function buildColumns(onRevoke: (id: string) => void): ColumnDef<SessionRow>[] {
  return [
    {
      accessorKey: "id",
      header: "Sessiya",
      meta: { cardTitle: true },
      cell: ({ row }) => <span>{row.original.id.slice(0, 8)}…</span>,
    },
    {
      accessorKey: "active",
      header: "Holat",
      cell: ({ row }) => <span>{row.original.active ? "faol" : "tugagan"}</span>,
    },
    {
      id: "actions",
      header: "Amallar",
      cell: ({ row }) =>
        row.original.active ? (
          <button type="button" onClick={() => onRevoke(row.original.id)}>
            Bekor qilish
          </button>
        ) : null,
    },
  ];
}

function renderCards(onRevoke: (id: string) => void) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      <AdminDataTable
        data={rows}
        columns={buildColumns(onRevoke)}
        emptyTitle="Bo‘sh"
        variant="cards"
        getRowId={(row) => row.id}
      />
    </NextIntlClientProvider>,
  );
}

describe("row actions in the card projection", () => {
  it("renders an untagged action column on the card", () => {
    renderCards(() => {});
    expect(screen.getAllByRole("button", { name: "Bekor qilish" })).toHaveLength(2);
  });

  it("fires the handler with the id of the row it belongs to", async () => {
    const user = userEvent.setup();
    const onRevoke = vi.fn();
    renderCards(onRevoke);

    // Reach the button through its own card, not by index, so a reordering
    // of the list cannot make this pass for the wrong row.
    const cards = screen.getAllByRole("listitem");
    const second = cards.find((card) => within(card).queryByText("sess-bbb…"));
    expect(second).toBeDefined();

    await user.click(within(second!).getByRole("button", { name: "Bekor qilish" }));

    expect(onRevoke).toHaveBeenCalledTimes(1);
    expect(onRevoke).toHaveBeenCalledWith("sess-bbbb");
  });

  it("still offers the action after the table is projected to cards", () => {
    // The same columns in table mode are the control: if the action were
    // missing here too, the card assertion above would prove nothing.
    render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <AdminDataTable
          data={rows}
          columns={buildColumns(() => {})}
          emptyTitle="Bo‘sh"
          variant="table"
          getRowId={(row) => row.id}
        />
      </NextIntlClientProvider>,
    );
    expect(screen.getAllByRole("button", { name: "Bekor qilish" })).toHaveLength(2);
  });
});

/**
 * The gate that actually ships. A permission-gated action must be removed as a
 * COLUMN, not merely emptied inside its cell — an emptied cell still leaves a
 * header on the table and a labelled blank on every card. This pins the
 * conditional-spread pattern used by payments/transactions and
 * payments/referral-payouts.
 */
describe("a permission-gated action column", () => {
  function columnsFor(canManage: boolean): ColumnDef<SessionRow>[] {
    return [
      {
        accessorKey: "id",
        header: "Sessiya",
        meta: { cardTitle: true },
        cell: ({ row }) => <span>{row.original.id.slice(0, 8)}…</span>,
      },
      ...(canManage
        ? ([
            {
              id: "actions",
              header: "Amallar",
              cell: () => <button type="button">Bekor qilish</button>,
            },
          ] as ColumnDef<SessionRow>[])
        : []),
    ];
  }

  function renderAs(canManage: boolean, variant: "cards" | "table") {
    return render(
      <NextIntlClientProvider locale="uz-Latn" messages={messages}>
        <AdminDataTable
          data={rows}
          columns={columnsFor(canManage)}
          emptyTitle="Bo‘sh"
          variant={variant}
          getRowId={(row) => row.id}
        />
      </NextIntlClientProvider>,
    );
  }

  it("shows the action to an operator who holds the permission", () => {
    renderAs(true, "cards");
    expect(screen.getAllByRole("button", { name: "Bekor qilish" })).toHaveLength(3);
  });

  it("leaves neither button nor label when the operator does not", () => {
    renderAs(false, "cards");
    expect(screen.queryByRole("button", { name: "Bekor qilish" })).not.toBeInTheDocument();
    // The header must be gone too — an empty labelled row is the bug.
    expect(screen.queryByText("Amallar")).not.toBeInTheDocument();
  });

  it("removes the column header from the desktop table as well", () => {
    renderAs(false, "table");
    expect(screen.queryByRole("columnheader", { name: "Amallar" })).not.toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "Sessiya" })).toBeInTheDocument();
  });
});
