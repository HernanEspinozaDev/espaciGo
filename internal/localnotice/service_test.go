package localnotice

import (
	"strings"
	"testing"
)

func TestRenderCloseNoticeEventsAvoidsSecretsAndUsesAuthenticatedContractLink(t *testing.T) {
	tests := []struct {
		typeName string
		id       string
		want     []string
	}{
		{"cuenta_bloqueada_login", "", []string{"Bloqueo temporal", "no incluye credenciales"}},
		{"verificacion_aprobada", "", []string{"verificación sintética aprobada", "inicia sesión"}},
		{"verificacion_rechazada", "", []string{"verificación sintética rechazada", "motivo estructurado"}},
		{"pago_arriendo_confirmado", "", []string{"Pago simulado confirmado", "SIN COBRO REAL"}},
		{"pago_arriendo_rechazado", "", []string{"Pago simulado rechazado", "SIN COBRO REAL"}},
		{"nueva_reserva_pagada", "", []string{"solicitud sintética", "SIN COBRO REAL"}},
		{"invitacion_firma", "018f1c0a-1234-4a00-8a00-0123456789ab", []string{"http://127.0.0.1:8081/?contract=018f1c0a-1234-4a00-8a00-0123456789ab", "no autentica"}},
		{"contrato_firmado", "", []string{"ambas firmas", "SIN VALIDEZ JURÍDICA"}},
	}
	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			subject, body := render(Notice{Type: tt.typeName, AggregateID: tt.id})
			for _, want := range tt.want {
				if !strings.Contains(strings.ToLower(subject+" "+body), strings.ToLower(want)) {
					t.Fatalf("notice %q/%q missing %q", subject, body, want)
				}
			}
			for _, forbidden := range []string{"Bearer ", "password=", "token=", "secret="} {
				if strings.Contains(strings.ToLower(subject+" "+body), strings.ToLower(forbidden)) {
					t.Fatalf("notice contains forbidden credential material %q", forbidden)
				}
			}
		})
	}
}
