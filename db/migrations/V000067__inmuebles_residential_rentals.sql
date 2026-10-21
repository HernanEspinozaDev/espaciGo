-- Residential rentals extend the existing category/profile catalog. Keep
-- common pricing, address, capacity and calendar fields in their current tables.
INSERT INTO public.categoria_espacio (codigo, nombre, orden)
VALUES ('inmuebles', 'Inmuebles', 9);

INSERT INTO public.categoria_perfil_atributos (categoria_codigo, version, perfil)
VALUES (
    'inmuebles',
    1,
    '{
      "category_code": "inmuebles",
      "schema_version": 1,
      "attributes": [
        {"code":"tipo_inmueble","label":"Tipo de inmueble","type":"enum","required":true,"filter_candidate":true,"order":1,"options":["Casa","Departamento","Pieza o habitación","Estudio o loft","Cabaña","Casa anexa","Dúplex","Otro"]},
        {"code":"tipo_de_espacio","label":"Qué se arrienda","type":"enum","required":true,"filter_candidate":true,"order":2,"options":["Inmueble completo","Habitación privada","Habitación compartida","Cama en habitación compartida"]},
        {"code":"superficie_construida_m2","label":"Superficie construida","type":"number","unit":"m²","minimum":0.01,"maximum":99999999.99,"step":0.01,"filter_candidate":true,"order":3},
        {"code":"superficie_terreno_m2","label":"Superficie del terreno","type":"number","unit":"m²","minimum":0.01,"maximum":99999999.99,"step":0.01,"filter_candidate":true,"order":4},
        {"code":"dormitorios","label":"Dormitorios","type":"integer","minimum":0,"maximum":2147483647,"filter_candidate":true,"order":5},
        {"code":"camas","label":"Camas","type":"integer","minimum":0,"maximum":2147483647,"filter_candidate":false,"order":6},
        {"code":"banos_completos","label":"Baños completos","type":"integer","minimum":0,"maximum":2147483647,"filter_candidate":true,"order":7},
        {"code":"medios_banos","label":"Medios baños","type":"integer","minimum":0,"maximum":2147483647,"filter_candidate":false,"order":8},
        {"code":"privacidad_bano","label":"Uso del baño","type":"enum","filter_candidate":true,"order":9,"options":["Privado en suite","Privado de uso exclusivo","Compartido","No disponible"]},
        {"code":"cocina","label":"Cocina","type":"enum","filter_candidate":false,"order":10,"options":["Completa","Kitchenette","Compartida","No disponible"]},
        {"code":"amoblado","label":"Amoblado","type":"boolean","filter_candidate":true,"order":11},
        {"code":"estacionamientos","label":"Estacionamientos","type":"integer","minimum":0,"maximum":2147483647,"filter_candidate":true,"order":12},
        {"code":"bodegas","label":"Bodegas","type":"integer","minimum":0,"maximum":2147483647,"filter_candidate":false,"order":13},
        {"code":"piso","label":"Piso de la unidad","type":"integer","minimum":0,"maximum":2147483647,"filter_candidate":false,"order":14},
        {"code":"pisos_edificio","label":"Pisos del edificio","type":"integer","minimum":1,"maximum":2147483647,"filter_candidate":false,"order":15},
        {"code":"ascensor","label":"Ascensor","type":"boolean","filter_candidate":true,"order":16},
        {"code":"balcon","label":"Balcón","type":"boolean","filter_candidate":true,"order":17},
        {"code":"terraza","label":"Terraza","type":"boolean","filter_candidate":true,"order":18},
        {"code":"patio_o_jardin","label":"Patio o jardín","type":"boolean","filter_candidate":true,"order":19},
        {"code":"piscina","label":"Piscina","type":"boolean","filter_candidate":true,"order":20},
        {"code":"wifi","label":"Wi-Fi","type":"boolean","filter_candidate":true,"order":21},
        {"code":"calefaccion","label":"Calefacción","type":"boolean","filter_candidate":true,"order":22},
        {"code":"aire_acondicionado","label":"Aire acondicionado","type":"boolean","filter_candidate":true,"order":23},
        {"code":"agua_caliente","label":"Agua caliente","type":"boolean","filter_candidate":false,"order":24},
        {"code":"lavadora","label":"Lavadora disponible","type":"boolean","filter_candidate":false,"order":25},
        {"code":"secadora","label":"Secadora disponible","type":"boolean","filter_candidate":false,"order":26},
        {"code":"refrigerador","label":"Refrigerador","type":"boolean","filter_candidate":false,"order":27},
        {"code":"servicios_incluidos","label":"Servicios incluidos","type":"enum_list","filter_candidate":false,"order":28,"options":["Agua","Electricidad","Gas","Internet","Gastos comunes","Limpieza","Ropa de cama"]},
        {"code":"mascotas","label":"Mascotas","type":"enum","filter_candidate":true,"order":29,"options":["Permitidas","No permitidas","Consultar"]},
        {"code":"visitas","label":"Visitas","type":"enum","filter_candidate":false,"order":30,"options":["Permitidas","Con restricciones","No permitidas"]},
        {"code":"fumadores","label":"Fumadores","type":"boolean","filter_candidate":false,"order":31},
        {"code":"fiestas_o_reuniones","label":"Fiestas o reuniones","type":"boolean","filter_candidate":false,"order":32},
        {"code":"duracion_minima_unidades","label":"Estadía mínima en la modalidad seleccionada","type":"integer","minimum":1,"maximum":2147483647,"filter_candidate":false,"order":33,"description":"Se expresa en la unidad de la tarifa base: horas, días o meses."},
        {"code":"duracion_maxima_unidades","label":"Estadía máxima en la modalidad seleccionada","type":"integer","minimum":1,"maximum":2147483647,"filter_candidate":false,"order":34,"description":"Se expresa en la unidad de la tarifa base: horas, días o meses."}
      ]
    }'::jsonb
);
