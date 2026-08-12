export function isAdminPaymentVoidable(row: {
  status: string;
  provider: string;
}): boolean {
  if (row.status === "failed") {
    return true;
  }
  return (
    row.provider === "manual" &&
    (row.status === "created" || row.status === "pending")
  );
}

export const ADMIN_PAYMENTS_DEFAULT_STATUS = "paid";
