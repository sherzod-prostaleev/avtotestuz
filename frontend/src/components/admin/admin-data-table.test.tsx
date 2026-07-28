import { describe, expect, it } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import type { ColumnDef } from "@tanstack/react-table";
import { AdminDataTable, type AdminColumnMeta } from "./admin-data-table";

type Row = { id: string; name: string; amount: number; secret: string };

const rows: Row[] = [
  { id: "1", name: "Musharraf Qodirova", amount: 49000, secret: "s1" },
  { id: "2", name: "Ali Valiyev", amount: 12000, secret: "s2" },
];

const columns: ColumnDef<Row, unknown>[] = [
  {
    accessorKey: "name",
    header: "Ism",
    meta: { cardTitle: true } satisfies AdminColumnMeta,
  },
  {
    accessorKey: "amount",
    header: "Summa",
    meta: { numeric: true } satisfies AdminColumnMeta,
  },
  {
    accessorKey: "secret",
    header: "Sirli",
    meta: { hideOnCard: true } satisfies AdminColumnMeta,
  },
];

const messages = {
  AdminTable: { rowsLabel: "Jadval qatorlari", cardsLabel: "Yozuvlar ro‘yxati" },
};

function renderTable(ui: React.ReactNode) {
  return render(
    <NextIntlClientProvider locale="uz-Latn" messages={messages}>
      {ui}
    </NextIntlClientProvider>,
  );
}

describe("AdminDataTable", () => {
  it("shows the empty state when there is no data", () => {
    renderTable(
      <AdminDataTable data={[]} columns={columns} emptyTitle="Bo‘sh" emptyDescription="Yo‘q" />,
    );
    expect(screen.getByText("Bo‘sh")).toBeInTheDocument();
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("renders a real table in table variant", () => {
    renderTable(
      <AdminDataTable data={rows} columns={columns} emptyTitle="Bo‘sh" variant="table" />,
    );
    const table = screen.getByRole("table");
    expect(within(table).getByText("Ism")).toBeInTheDocument();
    expect(within(table).getByText("Musharraf Qodirova")).toBeInTheDocument();
  });

  it("never forces a fixed minimum width that overflows phones", () => {
    const { container } = renderTable(
      <AdminDataTable data={rows} columns={columns} emptyTitle="Bo‘sh" variant="table" />,
    );
    expect(container.querySelector('[class*="min-w-[720px]"]')).toBeNull();
  });

  it("renders labelled cards in cards variant", () => {
    renderTable(
      <AdminDataTable data={rows} columns={columns} emptyTitle="Bo‘sh" variant="cards" />,
    );
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    const list = screen.getByRole("list", { name: "Yozuvlar ro‘yxati" });
    const items = within(list).getAllByRole("listitem");
    expect(items).toHaveLength(2);
    // cardTitle column renders as the heading, without its label
    expect(within(items[0]).getByText("Musharraf Qodirova")).toBeInTheDocument();
    // labelled row for a normal column
    expect(within(items[0]).getByText("Summa")).toBeInTheDocument();
    expect(within(items[0]).getByText("49000")).toBeInTheDocument();
  });

  it("omits hideOnCard columns from cards but keeps them in the table", () => {
    const cards = renderTable(
      <AdminDataTable data={rows} columns={columns} emptyTitle="Bo‘sh" variant="cards" />,
    );
    expect(screen.queryByText("Sirli")).not.toBeInTheDocument();
    cards.unmount();

    renderTable(
      <AdminDataTable data={rows} columns={columns} emptyTitle="Bo‘sh" variant="table" />,
    );
    expect(screen.getByText("Sirli")).toBeInTheDocument();
  });

  it("prefers a bespoke renderCard when supplied", () => {
    renderTable(
      <AdminDataTable
        data={rows}
        columns={columns}
        emptyTitle="Bo‘sh"
        variant="cards"
        renderCard={(row) => <span>custom:{row.name}</span>}
      />,
    );
    expect(screen.getByText("custom:Musharraf Qodirova")).toBeInTheDocument();
    expect(screen.queryByText("Summa")).not.toBeInTheDocument();
  });
});
