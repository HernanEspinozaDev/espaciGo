package migrator

import (
	"context"
	"path/filepath"
	"testing"
)

func TestM03MigrationKeepsVerificationSyntheticAndResolutionConsistent(t *testing.T) {
	withTestDatabase(t, func(ctx context.Context, databaseURL string) {
		if _, err := Run(ctx, databaseURL, filepath.Join("..", "..", "db", "migrations")); err != nil {
			t.Fatal(err)
		}
		conn := connectTestDB(t, ctx, databaseURL)
		defer conn.Close(context.Background())
		for _, name := range []string{"id", "usuario_id", "tipo", "estado", "proveedor_ref", "referencia_evidencia", "clave_idempotencia", "reintento_de", "revisor_id", "motivo_codigo", "creada_en", "resuelta_en"} {
			var exists bool
			if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='verificacion' AND column_name=$1)`, name).Scan(&exists); err != nil {
				t.Fatal(err)
			}
			if !exists {
				t.Errorf("missing verificacion.%s", name)
			}
		}
		var forbidden int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='verificacion' AND column_name IN ('rut','numero_serie','documento_binario','selfie')`).Scan(&forbidden); err != nil {
			t.Fatal(err)
		}
		if forbidden != 0 {
			t.Fatalf("M03 schema stores %d prohibited identity columns", forbidden)
		}
	})
}
