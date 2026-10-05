ALTER TABLE public.verificacion
    ADD CONSTRAINT verificacion_reason_code_ck CHECK (
        motivo_codigo IS NULL OR motivo_codigo IN (
            'documento_vencido', 'antecedentes_incompletos', 'inicio_actividades_no_confirmado'
        )
    );
