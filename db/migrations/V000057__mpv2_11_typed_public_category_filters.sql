-- MPV2-11 expands public search to reviewed, typed commercial profile fields.
-- Public metadata remains sourced from the immutable, versioned V6 profile;
-- membership in atributo_filtro_publico is the explicit exposure policy.
ALTER TABLE public.atributo_filtro_publico
    DROP CONSTRAINT IF EXISTS atributo_filtro_publico_tipo_check;

ALTER TABLE public.atributo_filtro_publico
    ADD CONSTRAINT atributo_filtro_publico_tipo_check
    CHECK (tipo IN ('boolean','integer','number','enum','enum_list'));

-- Keep the pre-existing boolean policy while adding only commercial fields
-- audited in plan_perfeccionamiento_publico.md. A temporary manifest lets the
-- migration fail closed if any code/profile/type is missing or ambiguous.
CREATE TEMPORARY TABLE mpv2_11_public_filter_manifest (
    categoria_codigo text NOT NULL,
    perfil_version integer NOT NULL,
    codigo text NOT NULL,
    PRIMARY KEY (categoria_codigo,perfil_version,codigo)
) ON COMMIT DROP;

INSERT INTO mpv2_11_public_filter_manifest(categoria_codigo,perfil_version,codigo) VALUES
('oficina',1,'wifi'),('oficina',1,'mobiliario_incluido'),
('oficina',1,'puestos_trabajo'),('oficina',1,'tipo_uso_oficina'),('oficina',1,'escritorios'),('oficina',1,'climatizacion'),
('sala_multiproposito',1,'wifi'),('sala_multiproposito',1,'proyector'),
('sala_multiproposito',1,'disposiciones_admitidas'),('sala_multiproposito',1,'sillas_disponibles'),('sala_multiproposito',1,'mesas_disponibles'),('sala_multiproposito',1,'pantalla'),('sala_multiproposito',1,'videoconferencia'),
('bodega',1,'carro_carga_disponible'),('bodega',1,'muelle_carga'),
('bodega',1,'altura_util_m'),('bodega',1,'volumen_util_m3'),('bodega',1,'acceso_carga'),('bodega',1,'ancho_acceso_m'),('bodega',1,'altura_acceso_m'),('bodega',1,'tipo_almacenamiento'),
('estacionamiento',1,'techado'),('estacionamiento',1,'carga_electrica'),
('estacionamiento',1,'largo_plaza_m'),('estacionamiento',1,'ancho_plaza_m'),('estacionamiento',1,'vehiculos_admitidos'),('estacionamiento',1,'altura_maxima_m'),('estacionamiento',1,'tipo_acceso'),
('local_flexible',1,'acceso_nivel_calle'),('local_flexible',1,'vitrina'),
('local_flexible',1,'frente_m'),('local_flexible',1,'probadores'),('local_flexible',1,'bodega_apoyo_m2'),('local_flexible',1,'climatizacion'),
('stand',1,'montaje_incluido'),('stand',1,'wifi'),
('stand',1,'ancho_stand_m'),('stand',1,'fondo_stand_m'),('stand',1,'lados_abiertos'),('stand',1,'altura_maxima_m'),('stand',1,'potencia_disponible_w'),('stand',1,'mesas_incluidas'),('stand',1,'sillas_incluidas'),
('quincho',1,'techado'),('quincho',1,'agua_potable'),
('quincho',1,'tipo_parrilla'),('quincho',1,'asientos_disponibles'),('quincho',1,'parrillas_disponibles'),('quincho',1,'mesas_disponibles'),('quincho',1,'lavaplatos'),('quincho',1,'refrigerador'),
('parcela_eventos',1,'salon_cubierto'),('parcela_eventos',1,'quincho_disponible'),
('parcela_eventos',1,'superficie_exterior_util_m2'),('parcela_eventos',1,'superficie_cubierta_util_m2'),('parcela_eventos',1,'estacionamientos_disponibles'),('parcela_eventos',1,'cocina_disponible'),('parcela_eventos',1,'piscina_disponible'),('parcela_eventos',1,'acceso_bus');

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM mpv2_11_public_filter_manifest m
        LEFT JOIN public.categoria_perfil_atributos p
          ON p.categoria_codigo=m.categoria_codigo AND p.version=m.perfil_version
        LEFT JOIN LATERAL jsonb_array_elements(COALESCE(p.perfil->'attributes','[]'::jsonb)) a(value)
          ON a.value->>'code'=m.codigo
        WHERE p.categoria_codigo IS NULL OR a.value IS NULL
           OR a.value->>'type' NOT IN ('boolean','integer','number','enum','enum_list')
           OR a.value->>'label' IS NULL OR a.value->>'order' IS NULL
    ) THEN
        RAISE EXCEPTION 'MPV2-11 public filter manifest is inconsistent with versioned profiles';
    END IF;
END $$;

-- Rebuild this small policy table from the complete reviewed manifest. Doing
-- so also avoids transient collisions in its unique display-order constraint
-- while old boolean rows move to their canonical profile order.
DELETE FROM public.atributo_filtro_publico;

INSERT INTO public.atributo_filtro_publico(categoria_codigo,perfil_version,codigo,etiqueta,tipo,orden)
SELECT m.categoria_codigo,m.perfil_version,m.codigo,a.value->>'label',a.value->>'type',(a.value->>'order')::smallint
FROM mpv2_11_public_filter_manifest m
JOIN public.categoria_perfil_atributos p
  ON p.categoria_codigo=m.categoria_codigo AND p.version=m.perfil_version
CROSS JOIN LATERAL jsonb_array_elements(p.perfil->'attributes') a(value)
WHERE a.value->>'code'=m.codigo;

-- Query by attribute key uses this operator class. The existing jsonb_path_ops
-- index remains for containment queries; numeric residual comparisons are
-- guarded by profile/type validation and the key-presence GIN index.
CREATE INDEX espacio_caracteristicas_public_filter_keys_idx
    ON public.espacio_caracteristicas USING gin (valores jsonb_ops);
