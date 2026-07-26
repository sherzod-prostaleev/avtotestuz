"use client";

import { createContext, useContext } from "react";

export type AdminMe = {
  id: string;
  email: string;
  display_name: string;
  roles: string[];
  permissions: string[];
  totp_enabled?: boolean;
  totp_setup_required?: boolean;
};

const AdminMeContext = createContext<AdminMe | null>(null);

export function AdminMeProvider({
  me,
  children,
}: {
  me: AdminMe;
  children: React.ReactNode;
}) {
  return <AdminMeContext.Provider value={me}>{children}</AdminMeContext.Provider>;
}

export function useAdminMe(): AdminMe {
  const me = useContext(AdminMeContext);
  if (!me) {
    throw new Error("useAdminMe must be used within AdminMeProvider");
  }
  return me;
}

export function useAdminMeOptional(): AdminMe | null {
  return useContext(AdminMeContext);
}
