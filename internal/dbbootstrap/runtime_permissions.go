package dbbootstrap

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// GrantRuntimePermissions applies the least-privilege table grants after migrations.
// Versioned rates and private simulations are append/read only for the API role.
func GrantRuntimePermissions(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `GRANT USAGE ON SCHEMA public TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.usuario, public.sesion, public.token_accion TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.rol_usuario, public.aceptacion_terminos TO espacigo_runtime;
 GRANT SELECT ON public.version_terminos TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.perfil_usuario TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.solicitud_titular TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.verificacion TO espacigo_runtime;
 GRANT SELECT, INSERT, DELETE ON public.verificacion_evidencia_sintetica TO espacigo_runtime;
 GRANT SELECT ON public.categoria_espacio, public.categoria_perfil_atributos TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.espacio TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE, DELETE ON public.espacio_horario_semanal, public.espacio_horario_semanal_tramo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE, DELETE ON public.espacio_caracteristicas TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.ocupacion TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.tarifa_espacio, public.simulacion_precio_privada TO espacigo_runtime;
 GRANT SELECT ON public.reserva_ensayo_local_fixture TO espacigo_runtime;
 GRANT SELECT ON public.reserva_ensayo_local_ubicacion_sintetica TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.cotizacion_reserva_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.reserva_ensayo_local TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_ensayo_transicion, public.reserva_pago_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.reserva_pago_ensayo_operacion TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_pago_evento_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.reserva_pago_evento_aplicacion_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_cancelacion_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.reserva_devolucion_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_devolucion_intento_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.mensaje_reserva_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.reserva_mensaje_lectura TO espacigo_runtime;`)
	return err
}
