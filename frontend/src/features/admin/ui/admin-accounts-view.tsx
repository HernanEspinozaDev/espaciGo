"use client";

import { useEffect, useState } from "react";
import {
  AlertCircle,
  AlertTriangle,
  CheckCircle2,
  Edit3,
  Loader2,
  Lock,
  LogOut,
  RefreshCw,
  Search,
  Shield,
  Unlock,
  Users,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/workspace/ui/card";
import { Button } from "@/components/workspace/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/workspace/ui/label";
import { Input } from "@/components/workspace/ui/input";
import { useAdmin } from "./use-admin";
import { isValidAdminReason } from "../domain/admin-rules";
import type { AdminRole, GlobalAdminAccount } from "../domain/types";
import { AdminFeedbackBanners } from "./admin-feedback-banners";

function renderAccountStateBadge(
  state: GlobalAdminAccount["state"],
  isBlocked: boolean,
) {
  if (isBlocked) {
    return (
      <Badge
        variant="destructive"
        className="gap-1 bg-red-100 text-red-800 dark:bg-red-950/50 dark:text-red-200 font-medium"
      >
        <Lock className="size-3.5" aria-hidden="true" />
        Bloqueada
      </Badge>
    );
  }
  switch (state) {
    case "activo":
      return (
        <Badge variant="default" className="gap-1 bg-emerald-700 text-white">
          <CheckCircle2 className="size-3.5" aria-hidden="true" />
          Activo
        </Badge>
      );
    case "correo_pendiente":
      return (
        <Badge variant="secondary" className="bg-amber-100 text-amber-800">
          Correo pendiente
        </Badge>
      );
    case "bloqueado":
      return (
        <Badge
          variant="destructive"
          className="bg-red-100 text-red-800 dark:bg-red-950/50 dark:text-red-200 font-medium"
        >
          Bloqueado
        </Badge>
      );
    case "baja_solicitada":
      return (
        <Badge variant="outline" className="text-rose-600 border-rose-300">
          Baja solicitada
        </Badge>
      );
    case "desidentificado":
      return (
        <Badge variant="outline" className="text-muted-foreground">
          Desidentificado
        </Badge>
      );
    default:
      return <Badge variant="outline">{state}</Badge>;
  }
}

export function AdminAccountsView() {
  const {
    snapshot,
    loadGlobalAccounts,
    setGlobalAccountRole,
    setGlobalLegacyAdminRole,
    setGlobalAccountBlocked,
    revokeGlobalAccountSessions,
    updateGlobalAccountProfile,
  } = useAdmin();

  const [searchQuery, setSearchQuery] = useState("");
  const [roleFilter, setRoleFilter] = useState<string>("todos");

  // Role assignment modal
  const [accountForRole, setAccountForRole] =
    useState<GlobalAdminAccount | null>(null);
  const [targetRole, setTargetRole] = useState<AdminRole>("arrendatario");
  const [grantAction, setGrantAction] = useState<boolean>(true);
  const [roleReason, setRoleReason] = useState<
    "correccion_perfil" | "revision_administrativa" | "gestion_operativa"
  >("gestion_operativa");

  // Block / Unblock modal
  const [accountForBlock, setAccountForBlock] =
    useState<GlobalAdminAccount | null>(null);
  const [blockAction, setBlockAction] = useState<boolean>(true);
  const [blockReason, setBlockReason] = useState("");
  const [blockError, setBlockError] = useState<string | null>(null);

  // Revoke sessions modal
  const [accountForRevoke, setAccountForRevoke] =
    useState<GlobalAdminAccount | null>(null);
  const [revokeReason, setRevokeReason] = useState<
    "correccion_perfil" | "revision_administrativa" | "gestion_operativa"
  >("revision_administrativa");

  // Edit profile modal
  const [accountForEdit, setAccountForEdit] =
    useState<GlobalAdminAccount | null>(null);
  const [editDisplayName, setEditDisplayName] = useState("");
  const [editPhone, setEditPhone] = useState("");
  const [editReason, setEditReason] = useState<
    "correccion_perfil" | "revision_administrativa" | "gestion_operativa"
  >("correccion_perfil");

  useEffect(() => {
    void loadGlobalAccounts();
  }, [loadGlobalAccounts]);

  const { globalAccounts, loading, actionLoading } = snapshot;

  const filteredAccounts = globalAccounts.filter((acc) => {
    const matchesRole =
      roleFilter === "todos" || acc.roles.includes(roleFilter as AdminRole);
    const matchesSearch =
      searchQuery.trim() === "" ||
      acc.email.toLowerCase().includes(searchQuery.toLowerCase()) ||
      (acc.display_name &&
        acc.display_name.toLowerCase().includes(searchQuery.toLowerCase())) ||
      acc.account_id.toLowerCase().includes(searchQuery.toLowerCase());
    return matchesRole && matchesSearch;
  });

  const handleOpenRoleModal = (acc: GlobalAdminAccount) => {
    setAccountForRole(acc);
    setTargetRole("arrendatario");
    setGrantAction(true);
    setRoleReason("gestion_operativa");
  };

  const handleConfirmRole = async () => {
    if (!accountForRole) return;
    let success = false;
    if (
      targetRole === "administrador" ||
      targetRole === "administrador_global"
    ) {
      success = await setGlobalLegacyAdminRole(
        accountForRole.account_id,
        grantAction,
        "revision_administrativa",
      );
    } else {
      success = await setGlobalAccountRole(
        accountForRole.account_id,
        targetRole as "arrendatario" | "arrendador",
        grantAction,
        roleReason,
      );
    }
    if (success) {
      setAccountForRole(null);
      await loadGlobalAccounts();
    }
  };

  const handleOpenBlockModal = (acc: GlobalAdminAccount, blocked: boolean) => {
    setAccountForBlock(acc);
    setBlockAction(blocked);
    setBlockReason("");
    setBlockError(null);
  };

  const handleConfirmBlock = async () => {
    if (!accountForBlock) return;
    if (!isValidAdminReason(blockReason)) {
      setBlockError(
        "El motivo del cambio de bloqueo es obligatorio y debe tener entre 5 y 500 caracteres válidos.",
      );
      return;
    }
    const success = await setGlobalAccountBlocked(
      accountForBlock.account_id,
      blockAction,
      blockReason.trim(),
    );
    if (success) {
      setAccountForBlock(null);
      await loadGlobalAccounts();
    }
  };

  const handleOpenRevokeModal = (acc: GlobalAdminAccount) => {
    setAccountForRevoke(acc);
    setRevokeReason("revision_administrativa");
  };

  const handleConfirmRevokeSessions = async () => {
    if (!accountForRevoke) return;
    const success = await revokeGlobalAccountSessions(
      accountForRevoke.account_id,
      revokeReason,
    );
    if (success) {
      setAccountForRevoke(null);
    }
  };

  const handleOpenEditProfile = (acc: GlobalAdminAccount) => {
    setAccountForEdit(acc);
    setEditDisplayName(acc.display_name ?? "");
    setEditPhone(acc.phone ?? "");
    setEditReason("correccion_perfil");
  };

  const handleConfirmEditProfile = async () => {
    if (!accountForEdit) return;
    const expected_updated_at =
      accountForEdit.profile_updated_at ?? new Date().toISOString();
    const success = await updateGlobalAccountProfile(
      accountForEdit.account_id,
      {
        display_name: editDisplayName.trim(),
        phone: editPhone.trim() || null,
        expected_updated_at,
        reason: editReason,
      },
    );
    if (success) {
      setAccountForEdit(null);
      await loadGlobalAccounts();
    }
  };

  return (
    <div className="space-y-6">
      <AdminFeedbackBanners />

      {/* Encabezado */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight text-foreground">
            Gestión de Cuentas y Asignación de Roles
          </h2>
          <p className="text-sm text-muted-foreground mt-0.5">
            Directorio de usuarios, control RBAC, bloqueo administrativo y
            revocación de sesiones.
          </p>
        </div>

        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => void loadGlobalAccounts()}
          disabled={loading}
          className="gap-1.5"
        >
          <RefreshCw
            className={`size-4 ${loading ? "animate-spin" : ""}`}
            aria-hidden="true"
          />
          <span>Actualizar</span>
        </Button>
      </div>

      {/* Búsqueda y Filtros */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
        <div className="relative flex-1">
          <Search
            className="absolute left-3 top-2.5 size-4 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            type="search"
            placeholder="Buscar por email, nombre o ID de cuenta..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9"
          />
        </div>

        <div className="flex items-center gap-2">
          <Label htmlFor="role-filter" className="sr-only">
            Filtrar por rol
          </Label>
          <select
            id="role-filter"
            value={roleFilter}
            onChange={(e) => setRoleFilter(e.target.value)}
            className="rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
          >
            <option value="todos">Todos los roles</option>
            <option value="arrendatario">Arrendatario</option>
            <option value="arrendador">Arrendador</option>
            <option value="administrador">Administrador</option>
            <option value="administrador_global">Admin Global</option>
          </select>
        </div>
      </div>

      {/* Lista de Cuentas */}
      {filteredAccounts.length === 0 ? (
        <Card>
          <CardContent className="py-12 text-center text-muted-foreground text-sm space-y-2">
            <Users
              className="size-10 mx-auto text-muted-foreground/60"
              aria-hidden="true"
            />
            <p className="font-medium">No se encontraron cuentas.</p>
          </CardContent>
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-4">
          {filteredAccounts.map((acc) => (
            <Card
              key={acc.account_id}
              className="shadow-sm hover:shadow transition-shadow"
            >
              <CardHeader className="pb-3 bg-muted/20 border-b border-border/40">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="space-y-0.5">
                    <CardTitle className="text-base font-semibold text-foreground">
                      {acc.display_name || "Usuario sin nombre configurado"}
                    </CardTitle>
                    <div className="flex items-center gap-2 text-xs text-muted-foreground font-mono">
                      <span>{acc.email}</span>
                      <span>•</span>
                      <span>ID: {acc.account_id.slice(0, 8)}...</span>
                    </div>
                  </div>
                  <div>
                    {renderAccountStateBadge(
                      acc.state,
                      acc.administrative_blocked,
                    )}
                  </div>
                </div>
              </CardHeader>
              <CardContent className="pt-4 space-y-4">
                {/* Resumen de actividad y roles */}
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-xs">
                  <div>
                    <span className="text-muted-foreground block">
                      Roles Activos
                    </span>
                    <div className="flex flex-wrap gap-1 mt-1">
                      {acc.roles.map((r) => (
                        <Badge
                          key={r}
                          variant="outline"
                          className="capitalize font-normal text-[11px]"
                        >
                          {r.replace(/_/g, " ")}
                        </Badge>
                      ))}
                    </div>
                  </div>
                  <div>
                    <span className="text-muted-foreground block">
                      Teléfono
                    </span>
                    <span className="font-medium block mt-1">
                      {acc.phone ?? "No registrado"}
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block">
                      Publicaciones / Reservas
                    </span>
                    <span className="font-medium block mt-1">
                      {acc.publication_count} pubs • {acc.reservation_count}{" "}
                      res.
                    </span>
                  </div>
                  <div>
                    <span className="text-muted-foreground block">
                      Registrado
                    </span>
                    <span className="font-medium block mt-1">
                      {new Date(acc.created_at).toLocaleDateString("es-CL")}
                    </span>
                  </div>
                </div>

                {/* Acciones de Cuenta */}
                <div className="flex flex-wrap items-center justify-end gap-2 pt-2 border-t border-border/40">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => handleOpenEditProfile(acc)}
                    className="text-xs gap-1.5"
                  >
                    <Edit3 className="size-3.5" aria-hidden="true" />
                    <span>Editar Perfil</span>
                  </Button>

                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => handleOpenRoleModal(acc)}
                    className="text-xs gap-1.5"
                  >
                    <Shield className="size-3.5" aria-hidden="true" />
                    <span>Modificar Roles</span>
                  </Button>

                  {acc.administrative_blocked ? (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => handleOpenBlockModal(acc, false)}
                      className="text-emerald-700 hover:bg-emerald-50 dark:text-emerald-400 text-xs gap-1.5 border-emerald-300"
                    >
                      <Unlock className="size-3.5" aria-hidden="true" />
                      <span>Desbloquear</span>
                    </Button>
                  ) : (
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => handleOpenBlockModal(acc, true)}
                      className="text-amber-700 hover:bg-amber-50 dark:text-amber-300 text-xs gap-1.5 border-amber-300"
                    >
                      <Lock className="size-3.5" aria-hidden="true" />
                      <span>Bloquear</span>
                    </Button>
                  )}

                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={() => handleOpenRevokeModal(acc)}
                    className="text-destructive hover:bg-destructive/10 border-destructive/30 text-xs gap-1.5"
                  >
                    <LogOut className="size-3.5" aria-hidden="true" />
                    <span>Revocar Sesiones</span>
                  </Button>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Modal Modificación de Roles */}
      <Dialog
        open={accountForRole !== null}
        onOpenChange={(open) => !open && setAccountForRole(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Shield className="size-5 text-primary" aria-hidden="true" />
              <span>Asignación de Roles RBAC</span>
            </DialogTitle>
            <DialogDescription>
              Otorga o revoca permisos y capacidades para{" "}
              {accountForRole?.email}.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="target-role">Rol a gestionar</Label>
              <select
                id="target-role"
                value={targetRole}
                onChange={(e) => setTargetRole(e.target.value as AdminRole)}
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="arrendatario">
                  Arrendatario (Usuario estándar)
                </option>
                <option value="arrendador">
                  Arrendador (Publicar espacios)
                </option>
                <option value="administrador">Administrador Operativo</option>
              </select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="grant-action">Acción</Label>
              <select
                id="grant-action"
                value={grantAction ? "true" : "false"}
                onChange={(e) => setGrantAction(e.target.value === "true")}
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="true">Conceder rol</option>
                <option value="false">Revocar rol</option>
              </select>
            </div>

            <div className="space-y-2">
              <Label htmlFor="role-reason">Motivo administrativo</Label>
              <select
                id="role-reason"
                value={roleReason}
                onChange={(e) =>
                  setRoleReason(
                    e.target.value as
                      | "correccion_perfil"
                      | "revision_administrativa"
                      | "gestion_operativa",
                  )
                }
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="gestion_operativa">Gestión operativa</option>
                <option value="revision_administrativa">
                  Revisión administrativa
                </option>
                <option value="correccion_perfil">Corrección de perfil</option>
              </select>
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setAccountForRole(null)}
              disabled={actionLoading}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              onClick={handleConfirmRole}
              disabled={actionLoading}
            >
              {actionLoading ? (
                <Loader2
                  className="size-4 animate-spin mr-1.5"
                  aria-hidden="true"
                />
              ) : null}
              Guardar Rol
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Modal Bloqueo / Desbloqueo */}
      <Dialog
        open={accountForBlock !== null}
        onOpenChange={(open) => !open && setAccountForBlock(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              {blockAction ? (
                <>
                  <Lock
                    className="size-5 text-destructive"
                    aria-hidden="true"
                  />
                  <span>Bloquear Cuenta de Usuario</span>
                </>
              ) : (
                <>
                  <Unlock
                    className="size-5 text-emerald-600"
                    aria-hidden="true"
                  />
                  <span>Desbloquear Cuenta de Usuario</span>
                </>
              )}
            </DialogTitle>
            <DialogDescription>
              {blockAction
                ? "El usuario no podrá autenticarse ni realizar reservas o publicaciones mientras esté bloqueado."
                : "Se restablecerá el acceso normal para este usuario."}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="block-reason">
                Motivo obligatorio (mínimo 5 caracteres)
              </Label>
              <Input
                id="block-reason"
                value={blockReason}
                onChange={(e) => {
                  setBlockReason(e.target.value);
                  if (blockError) setBlockError(null);
                }}
                placeholder="Ej. Actividad sospechosa o solicitud de bloqueo por prevención..."
                aria-required="true"
                aria-invalid={blockError !== null}
                className={blockError ? "border-destructive" : ""}
              />
              {blockError && (
                <p className="text-xs text-destructive flex items-center gap-1 mt-1">
                  <AlertCircle className="size-3.5" aria-hidden="true" />
                  {blockError}
                </p>
              )}
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setAccountForBlock(null)}
              disabled={actionLoading}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              variant={blockAction ? "destructive" : "default"}
              onClick={handleConfirmBlock}
              disabled={actionLoading}
              className={
                !blockAction
                  ? "bg-emerald-700 hover:bg-emerald-800 text-white"
                  : ""
              }
            >
              {actionLoading ? (
                <Loader2
                  className="size-4 animate-spin mr-1.5"
                  aria-hidden="true"
                />
              ) : null}
              Confirmar {blockAction ? "Bloqueo" : "Desbloqueo"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Modal Revocar Sesiones Activas */}
      <Dialog
        open={accountForRevoke !== null}
        onOpenChange={(open) => !open && setAccountForRevoke(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <AlertTriangle
                className="size-5 text-destructive"
                aria-hidden="true"
              />
              <span>Revocar Sesiones Activas</span>
            </DialogTitle>
            <DialogDescription>
              Aviso de acción irreversible: Se invalidarán de inmediato todos
              los tokens de acceso y refresco de {accountForRevoke?.email}. Si
              revocas las sesiones de tu propia cuenta, tus datos privados en
              memoria se purgarán de inmediato.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="revoke-reason">
                Motivo administrativo de revocación
              </Label>
              <select
                id="revoke-reason"
                value={revokeReason}
                onChange={(e) =>
                  setRevokeReason(
                    e.target.value as
                      | "correccion_perfil"
                      | "revision_administrativa"
                      | "gestion_operativa",
                  )
                }
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="revision_administrativa">
                  Revisión administrativa
                </option>
                <option value="gestion_operativa">Gestión operativa</option>
                <option value="correccion_perfil">Corrección de perfil</option>
              </select>
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setAccountForRevoke(null)}
              disabled={actionLoading}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={handleConfirmRevokeSessions}
              disabled={actionLoading}
            >
              {actionLoading ? (
                <Loader2
                  className="size-4 animate-spin mr-1.5"
                  aria-hidden="true"
                />
              ) : null}
              Confirmar Revocación Irreversible
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Modal Editar Perfil con Control de Concurrencia */}
      <Dialog
        open={accountForEdit !== null}
        onOpenChange={(open) => !open && setAccountForEdit(null)}
      >
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>Editar Perfil Administrativo</DialogTitle>
            <DialogDescription>
              Actualización directa con control de concurrencia optimista
              (expected_updated_at).
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4 py-2">
            <div className="space-y-2">
              <Label htmlFor="edit-display-name">Nombre para mostrar</Label>
              <Input
                id="edit-display-name"
                value={editDisplayName}
                onChange={(e) => setEditDisplayName(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="edit-phone">Teléfono</Label>
              <Input
                id="edit-phone"
                value={editPhone}
                onChange={(e) => setEditPhone(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="edit-reason">Motivo administrativo</Label>
              <select
                id="edit-reason"
                value={editReason}
                onChange={(e) =>
                  setEditReason(
                    e.target.value as
                      | "correccion_perfil"
                      | "revision_administrativa"
                      | "gestion_operativa",
                  )
                }
                className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
              >
                <option value="correccion_perfil">Corrección de perfil</option>
                <option value="revision_administrativa">
                  Revisión administrativa
                </option>
                <option value="gestion_operativa">Gestión operativa</option>
              </select>
            </div>
          </div>

          <DialogFooter className="gap-2 sm:gap-0">
            <Button
              type="button"
              variant="outline"
              onClick={() => setAccountForEdit(null)}
              disabled={actionLoading}
            >
              Cancelar
            </Button>
            <Button
              type="button"
              onClick={handleConfirmEditProfile}
              disabled={actionLoading}
            >
              {actionLoading ? (
                <Loader2
                  className="size-4 animate-spin mr-1.5"
                  aria-hidden="true"
                />
              ) : null}
              Guardar Cambios
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
