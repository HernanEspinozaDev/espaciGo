import type { Metadata } from "next";
import { AdminAccountsView } from "@/features/admin/ui/admin-accounts-view";

export const metadata: Metadata = {
  title: "Cuentas y Roles | Administración | EspaciGo",
  description:
    "Directorio administrativo de cuentas, asignación de roles RBAC y control de acceso.",
  robots: {
    index: false,
    follow: false,
  },
};

export default function AdministracionCuentasPage() {
  return <AdminAccountsView />;
}
