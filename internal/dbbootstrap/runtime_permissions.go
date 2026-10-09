package dbbootstrap

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// GrantRuntimePermissions applies the least-privilege table grants after migrations.
// Versioned rates and private simulations are append/read only for user flows;
// the administrator-guarded local suppression path may delete private simulations.
func GrantRuntimePermissions(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `GRANT USAGE ON SCHEMA public TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.usuario, public.sesion, public.token_accion TO espacigo_runtime;
 GRANT DELETE ON public.sesion, public.token_accion TO espacigo_runtime;
 GRANT SELECT, INSERT, DELETE ON public.historial_clave_local TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.outbox_evento_local TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.outbox_evento_ciclo_local TO espacigo_runtime;
 GRANT EXECUTE ON FUNCTION public.purge_expired_local_credential_notices(timestamptz,integer) TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.evento_auditoria_local TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.disputa_ensayo_local TO espacigo_runtime;
 GRANT UPDATE (estado,cerrada_por,motivo_cierre_codigo,cerrada_en,anfitrion_id,arrendatario_id,abierta_por) ON public.disputa_ensayo_local TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.disputa_ensayo_historial TO espacigo_runtime;
 GRANT UPDATE (actor_id) ON public.disputa_ensayo_historial TO espacigo_runtime;
 GRANT USAGE, SELECT ON SEQUENCE public.disputa_ensayo_historial_secuencia_seq TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.rol_usuario, public.aceptacion_terminos TO espacigo_runtime;
 GRANT DELETE ON public.rol_usuario TO espacigo_runtime;
 GRANT UPDATE (concedido_por) ON public.rol_usuario TO espacigo_runtime;
 GRANT UPDATE (retirar_en) ON public.aceptacion_terminos TO espacigo_runtime;
 GRANT SELECT ON public.version_terminos TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE, DELETE ON public.perfil_usuario TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE (estado,resuelta_en,motivo_resolucion_codigo,retirar_en) ON public.solicitud_titular TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.verificacion TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.verificacion_historial_local TO espacigo_runtime;
 GRANT USAGE, SELECT ON SEQUENCE public.verificacion_historial_local_id_seq TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.elegibilidad_verificacion_local TO espacigo_runtime;
 GRANT SELECT, INSERT, DELETE ON public.verificacion_evidencia_sintetica TO espacigo_runtime;
 GRANT SELECT ON public.categoria_espacio, public.categoria_perfil_atributos TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.espacio TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.espacio_publicacion_historial_local TO espacigo_runtime;
	GRANT USAGE, SELECT ON SEQUENCE public.espacio_publicacion_historial_local_id_seq TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.espacio_galeria_sintetica_local TO espacigo_runtime;
	GRANT UPDATE (estado,retirada_en,proximo_intento_en,limpia_en,intentos_limpieza,ultimo_codigo_error,archivo_id) ON public.espacio_galeria_sintetica_local TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE, DELETE ON public.espacio_galeria_archivo_candidato_local TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE, DELETE ON public.espacio_horario_semanal, public.espacio_horario_semanal_tramo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE, DELETE ON public.espacio_caracteristicas TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.ocupacion TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.tarifa_espacio, public.simulacion_precio_privada TO espacigo_runtime;
 GRANT SELECT ON public.reserva_ensayo_local_fixture TO espacigo_runtime;
 GRANT UPDATE (habilitada,anfitrion_id,arrendatario_id) ON public.reserva_ensayo_local_fixture TO espacigo_runtime;
 GRANT SELECT ON public.reserva_ensayo_local_ubicacion_sintetica TO espacigo_runtime;
 GRANT SELECT, INSERT, DELETE ON public.cotizacion_reserva_ensayo TO espacigo_runtime;
 GRANT UPDATE (retirar_en,anfitrion_id,arrendatario_id) ON public.cotizacion_reserva_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.reserva_ensayo_local TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_ensayo_transicion, public.reserva_pago_ensayo TO espacigo_runtime;
	GRANT UPDATE (actor_id) ON public.reserva_ensayo_transicion TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE ON public.reserva_pago_ensayo_operacion TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE ON public.reserva_pago_fake_resultado_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_pago_evento_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.reserva_pago_evento_aplicacion_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_cancelacion_ensayo TO espacigo_runtime;
	GRANT UPDATE (arrendatario_id) ON public.reserva_cancelacion_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.reserva_devolucion_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_devolucion_intento_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.contrato_ensayo_local, public.contrato_ensayo_firma TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.documento_privado_sintetico_local TO espacigo_runtime;
 GRANT UPDATE (estado,actualizada_en,firmado_en) ON public.contrato_ensayo_local TO espacigo_runtime;
 GRANT UPDATE (estado,motivo,actualizada_en) ON public.contrato_ensayo_firma TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.contrato_ensayo_historial TO espacigo_runtime;
 GRANT SELECT, INSERT, DELETE ON public.mensaje_reserva_ensayo TO espacigo_runtime;
	GRANT UPDATE (autor_id) ON public.mensaje_reserva_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE, DELETE ON public.reserva_mensaje_lectura TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.ejecucion_baja_local TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.baja_archivo_pendiente_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.foto_perfil_sintetica_local TO espacigo_runtime;
	GRANT UPDATE (estado,retirada_en,limpia_en,intentos_limpieza,proximo_intento_en,ultimo_codigo_error,archivo_id,mime_type,sha256,size_bytes) ON public.foto_perfil_sintetica_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.cuenta_cobro_sintetica_local TO espacigo_runtime;
	GRANT UPDATE (referencia_ficticia,estado,actualizada_en,revocada_en) ON public.cuenta_cobro_sintetica_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.cuenta_cobro_sintetica_historial_local TO espacigo_runtime;
	GRANT USAGE, SELECT ON SEQUENCE public.cuenta_cobro_sintetica_historial_local_id_seq TO espacigo_runtime;
	GRANT SELECT, INSERT, DELETE ON public.m02_operacion_idempotente_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.reserva_vinculo_purgado_local TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE ON public.reaplicacion_baja_local TO espacigo_runtime;
	`)
	if err != nil {
		return err
	}
	// The API's administrator-guarded local suppression path needs to remove
	// private simulations. No pricing endpoint exposes that delete operation.
	_, err = conn.Exec(ctx, `GRANT DELETE ON public.simulacion_precio_privada TO espacigo_runtime;`)
	return err
}
