export type WorkspaceActivity = {
  id: string;
  title: string;
  description: string;
  icon: "view" | "account" | "security";
  when: string;
};

export type WorkspaceSummary = {
  title: string;
  value: string;
  detail: string;
};

export const workspaceSummary: WorkspaceSummary[] = [
  { title: "Reservas", value: "—", detail: "Sin conexión a reservas" },
  { title: "Espacios", value: "—", detail: "Sin cuenta de anfitrión" },
  { title: "Actividad", value: "—", detail: "Sin sesión conectada" },
];

export const workspaceActivity: WorkspaceActivity[] = [
  {
    id: "demo-activity-1",
    title: "Vista de seguridad",
    description: "Evento ilustrativo del panel",
    icon: "security",
    when: "Muestra",
  },
  {
    id: "demo-activity-2",
    title: "Ficha de espacio",
    description: "Evento ilustrativo de navegación",
    icon: "view",
    when: "Muestra",
  },
  {
    id: "demo-activity-3",
    title: "Preferencias de cuenta",
    description: "Evento ilustrativo de configuración",
    icon: "account",
    when: "Muestra",
  },
];

export const demoAccount = {
  name: "Cuenta de ejemplo",
  email: "contacto@example.invalid",
  role: "Sin sesión",
} as const;
