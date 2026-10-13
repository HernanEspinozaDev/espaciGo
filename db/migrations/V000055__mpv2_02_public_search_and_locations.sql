-- MPV2-02: versioned public location vocabulary and PostgreSQL Spanish FTS.
-- Location data is the DPA/CUT 2018 code/name snapshot; existing offers stay NULL.
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE TEXT SEARCH DICTIONARY public.espacigo_es_unaccent_dict (
    TEMPLATE = public.unaccent,
    RULES = 'unaccent'
);
CREATE TEXT SEARCH CONFIGURATION public.espacigo_es_unaccent (COPY = pg_catalog.spanish);
ALTER TEXT SEARCH CONFIGURATION public.espacigo_es_unaccent
    ALTER MAPPING FOR asciiword, asciihword, hword, hword_asciipart, hword_part, word
    WITH public.espacigo_es_unaccent_dict, pg_catalog.spanish_stem;

CREATE TABLE public.catalogo_ubicacion_publica (
    version text PRIMARY KEY CHECK (version ~ '^[A-Z0-9_]{1,40}$'),
    fuente text NOT NULL,
    publicado_en date NOT NULL,
    activo boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX catalogo_ubicacion_publica_activo_idx
    ON public.catalogo_ubicacion_publica (activo) WHERE activo;

CREATE TABLE public.region_publica (
    catalogo_version text NOT NULL REFERENCES public.catalogo_ubicacion_publica(version),
    codigo text NOT NULL CHECK (codigo ~ '^[0-9]{2}$'),
    nombre text NOT NULL CHECK (length(btrim(nombre)) BETWEEN 1 AND 100),
    PRIMARY KEY (catalogo_version, codigo),
    UNIQUE (catalogo_version, codigo, nombre)
);

CREATE TABLE public.comuna_publica (
    catalogo_version text NOT NULL,
    codigo text NOT NULL CHECK (codigo ~ '^[0-9]{5}$'),
    region_codigo text NOT NULL,
    nombre text NOT NULL CHECK (length(btrim(nombre)) BETWEEN 1 AND 100),
    PRIMARY KEY (catalogo_version, codigo),
    UNIQUE (catalogo_version, region_codigo, codigo),
    FOREIGN KEY (catalogo_version, region_codigo)
        REFERENCES public.region_publica(catalogo_version, codigo)
);
CREATE INDEX comuna_publica_region_idx
    ON public.comuna_publica (catalogo_version, region_codigo, nombre COLLATE "C", codigo);

INSERT INTO public.catalogo_ubicacion_publica(version, fuente, publicado_en, activo)
VALUES ('SUBDERE_DPA_2018', 'SUBDERE CUT 2018 v04 / IDE Chile DPA 2018', DATE '2018-09-21', true);
INSERT INTO public.region_publica(catalogo_version,codigo,nombre) VALUES
('SUBDERE_DPA_2018','01','Tarapacá'),
('SUBDERE_DPA_2018','02','Antofagasta'),
('SUBDERE_DPA_2018','03','Atacama'),
('SUBDERE_DPA_2018','04','Coquimbo'),
('SUBDERE_DPA_2018','05','Valparaíso'),
('SUBDERE_DPA_2018','06','Libertador General Bernardo O''Higgins'),
('SUBDERE_DPA_2018','07','Maule'),
('SUBDERE_DPA_2018','08','Biobío'),
('SUBDERE_DPA_2018','09','La Araucanía'),
('SUBDERE_DPA_2018','10','Los Lagos'),
('SUBDERE_DPA_2018','11','Aysén del General Carlos Ibáñez del Campo'),
('SUBDERE_DPA_2018','12','Magallanes y de la Antártica Chilena'),
('SUBDERE_DPA_2018','13','Metropolitana de Santiago'),
('SUBDERE_DPA_2018','14','Los Ríos'),
('SUBDERE_DPA_2018','15','Arica y Parinacota'),
('SUBDERE_DPA_2018','16','Ñuble');
INSERT INTO public.comuna_publica(catalogo_version,region_codigo,codigo,nombre) VALUES
('SUBDERE_DPA_2018','01','01101','Iquique'),
('SUBDERE_DPA_2018','01','01107','Alto Hospicio'),
('SUBDERE_DPA_2018','01','01401','Pozo Almonte'),
('SUBDERE_DPA_2018','01','01402','Camiña'),
('SUBDERE_DPA_2018','01','01403','Colchane'),
('SUBDERE_DPA_2018','01','01404','Huara'),
('SUBDERE_DPA_2018','01','01405','Pica'),
('SUBDERE_DPA_2018','02','02101','Antofagasta'),
('SUBDERE_DPA_2018','02','02102','Mejillones'),
('SUBDERE_DPA_2018','02','02103','Sierra Gorda'),
('SUBDERE_DPA_2018','02','02104','Taltal'),
('SUBDERE_DPA_2018','02','02201','Calama'),
('SUBDERE_DPA_2018','02','02202','Ollagüe'),
('SUBDERE_DPA_2018','02','02203','San Pedro de Atacama'),
('SUBDERE_DPA_2018','02','02301','Tocopilla'),
('SUBDERE_DPA_2018','02','02302','María Elena'),
('SUBDERE_DPA_2018','03','03101','Copiapó'),
('SUBDERE_DPA_2018','03','03102','Caldera'),
('SUBDERE_DPA_2018','03','03103','Tierra Amarilla'),
('SUBDERE_DPA_2018','03','03201','Chañaral'),
('SUBDERE_DPA_2018','03','03202','Diego de Almagro'),
('SUBDERE_DPA_2018','03','03301','Vallenar'),
('SUBDERE_DPA_2018','03','03302','Alto del Carmen'),
('SUBDERE_DPA_2018','03','03303','Freirina'),
('SUBDERE_DPA_2018','03','03304','Huasco'),
('SUBDERE_DPA_2018','04','04101','La Serena'),
('SUBDERE_DPA_2018','04','04102','Coquimbo'),
('SUBDERE_DPA_2018','04','04103','Andacollo'),
('SUBDERE_DPA_2018','04','04104','La Higuera'),
('SUBDERE_DPA_2018','04','04105','Paiguano'),
('SUBDERE_DPA_2018','04','04106','Vicuña'),
('SUBDERE_DPA_2018','04','04201','Illapel'),
('SUBDERE_DPA_2018','04','04202','Canela'),
('SUBDERE_DPA_2018','04','04203','Los Vilos'),
('SUBDERE_DPA_2018','04','04204','Salamanca'),
('SUBDERE_DPA_2018','04','04301','Ovalle'),
('SUBDERE_DPA_2018','04','04302','Combarbalá'),
('SUBDERE_DPA_2018','04','04303','Monte Patria'),
('SUBDERE_DPA_2018','04','04304','Punitaqui'),
('SUBDERE_DPA_2018','04','04305','Río Hurtado'),
('SUBDERE_DPA_2018','05','05101','Valparaíso'),
('SUBDERE_DPA_2018','05','05102','Casablanca'),
('SUBDERE_DPA_2018','05','05103','Concón'),
('SUBDERE_DPA_2018','05','05104','Juan Fernández'),
('SUBDERE_DPA_2018','05','05105','Puchuncaví'),
('SUBDERE_DPA_2018','05','05107','Quintero'),
('SUBDERE_DPA_2018','05','05109','Viña del Mar'),
('SUBDERE_DPA_2018','05','05201','Isla de Pascua'),
('SUBDERE_DPA_2018','05','05301','Los Andes'),
('SUBDERE_DPA_2018','05','05302','Calle Larga'),
('SUBDERE_DPA_2018','05','05303','Rinconada'),
('SUBDERE_DPA_2018','05','05304','San Esteban'),
('SUBDERE_DPA_2018','05','05401','La Ligua'),
('SUBDERE_DPA_2018','05','05402','Cabildo'),
('SUBDERE_DPA_2018','05','05403','Papudo'),
('SUBDERE_DPA_2018','05','05404','Petorca'),
('SUBDERE_DPA_2018','05','05405','Zapallar'),
('SUBDERE_DPA_2018','05','05501','Quillota'),
('SUBDERE_DPA_2018','05','05502','Calera'),
('SUBDERE_DPA_2018','05','05503','Hijuelas'),
('SUBDERE_DPA_2018','05','05504','La Cruz'),
('SUBDERE_DPA_2018','05','05506','Nogales'),
('SUBDERE_DPA_2018','05','05601','San Antonio'),
('SUBDERE_DPA_2018','05','05602','Algarrobo'),
('SUBDERE_DPA_2018','05','05603','Cartagena'),
('SUBDERE_DPA_2018','05','05604','El Quisco'),
('SUBDERE_DPA_2018','05','05605','El Tabo'),
('SUBDERE_DPA_2018','05','05606','Santo Domingo'),
('SUBDERE_DPA_2018','05','05701','San Felipe'),
('SUBDERE_DPA_2018','05','05702','Catemu'),
('SUBDERE_DPA_2018','05','05703','Llaillay'),
('SUBDERE_DPA_2018','05','05704','Panquehue'),
('SUBDERE_DPA_2018','05','05705','Putaendo'),
('SUBDERE_DPA_2018','05','05706','Santa María'),
('SUBDERE_DPA_2018','05','05801','Quilpué'),
('SUBDERE_DPA_2018','05','05802','Limache'),
('SUBDERE_DPA_2018','05','05803','Olmué'),
('SUBDERE_DPA_2018','05','05804','Villa Alemana'),
('SUBDERE_DPA_2018','06','06101','Rancagua'),
('SUBDERE_DPA_2018','06','06102','Codegua'),
('SUBDERE_DPA_2018','06','06103','Coinco'),
('SUBDERE_DPA_2018','06','06104','Coltauco'),
('SUBDERE_DPA_2018','06','06105','Doñihue'),
('SUBDERE_DPA_2018','06','06106','Graneros'),
('SUBDERE_DPA_2018','06','06107','Las Cabras'),
('SUBDERE_DPA_2018','06','06108','Machalí'),
('SUBDERE_DPA_2018','06','06109','Malloa'),
('SUBDERE_DPA_2018','06','06110','Mostazal'),
('SUBDERE_DPA_2018','06','06111','Olivar'),
('SUBDERE_DPA_2018','06','06112','Peumo'),
('SUBDERE_DPA_2018','06','06113','Pichidegua'),
('SUBDERE_DPA_2018','06','06114','Quinta de Tilcoco'),
('SUBDERE_DPA_2018','06','06115','Rengo'),
('SUBDERE_DPA_2018','06','06116','Requínoa'),
('SUBDERE_DPA_2018','06','06117','San Vicente'),
('SUBDERE_DPA_2018','06','06201','Pichilemu'),
('SUBDERE_DPA_2018','06','06202','La Estrella'),
('SUBDERE_DPA_2018','06','06203','Litueche'),
('SUBDERE_DPA_2018','06','06204','Marchihue'),
('SUBDERE_DPA_2018','06','06205','Navidad'),
('SUBDERE_DPA_2018','06','06206','Paredones'),
('SUBDERE_DPA_2018','06','06301','San Fernando'),
('SUBDERE_DPA_2018','06','06302','Chépica'),
('SUBDERE_DPA_2018','06','06303','Chimbarongo'),
('SUBDERE_DPA_2018','06','06304','Lolol'),
('SUBDERE_DPA_2018','06','06305','Nancagua'),
('SUBDERE_DPA_2018','06','06306','Palmilla'),
('SUBDERE_DPA_2018','06','06307','Peralillo'),
('SUBDERE_DPA_2018','06','06308','Placilla'),
('SUBDERE_DPA_2018','06','06309','Pumanque'),
('SUBDERE_DPA_2018','06','06310','Santa Cruz'),
('SUBDERE_DPA_2018','07','07101','Talca'),
('SUBDERE_DPA_2018','07','07102','Constitución'),
('SUBDERE_DPA_2018','07','07103','Curepto'),
('SUBDERE_DPA_2018','07','07104','Empedrado'),
('SUBDERE_DPA_2018','07','07105','Maule'),
('SUBDERE_DPA_2018','07','07106','Pelarco'),
('SUBDERE_DPA_2018','07','07107','Pencahue'),
('SUBDERE_DPA_2018','07','07108','Río Claro'),
('SUBDERE_DPA_2018','07','07109','San Clemente'),
('SUBDERE_DPA_2018','07','07110','San Rafael'),
('SUBDERE_DPA_2018','07','07201','Cauquenes'),
('SUBDERE_DPA_2018','07','07202','Chanco'),
('SUBDERE_DPA_2018','07','07203','Pelluhue'),
('SUBDERE_DPA_2018','07','07301','Curicó'),
('SUBDERE_DPA_2018','07','07302','Hualañé'),
('SUBDERE_DPA_2018','07','07303','Licantén'),
('SUBDERE_DPA_2018','07','07304','Molina'),
('SUBDERE_DPA_2018','07','07305','Rauco'),
('SUBDERE_DPA_2018','07','07306','Romeral'),
('SUBDERE_DPA_2018','07','07307','Sagrada Familia'),
('SUBDERE_DPA_2018','07','07308','Teno'),
('SUBDERE_DPA_2018','07','07309','Vichuquén'),
('SUBDERE_DPA_2018','07','07401','Linares'),
('SUBDERE_DPA_2018','07','07402','Colbún'),
('SUBDERE_DPA_2018','07','07403','Longaví'),
('SUBDERE_DPA_2018','07','07404','Parral'),
('SUBDERE_DPA_2018','07','07405','Retiro'),
('SUBDERE_DPA_2018','07','07406','San Javier'),
('SUBDERE_DPA_2018','07','07407','Villa Alegre'),
('SUBDERE_DPA_2018','07','07408','Yerbas Buenas'),
('SUBDERE_DPA_2018','08','08101','Concepción'),
('SUBDERE_DPA_2018','08','08102','Coronel'),
('SUBDERE_DPA_2018','08','08103','Chiguayante'),
('SUBDERE_DPA_2018','08','08104','Florida'),
('SUBDERE_DPA_2018','08','08105','Hualqui'),
('SUBDERE_DPA_2018','08','08106','Lota'),
('SUBDERE_DPA_2018','08','08107','Penco'),
('SUBDERE_DPA_2018','08','08108','San Pedro de la Paz'),
('SUBDERE_DPA_2018','08','08109','Santa Juana'),
('SUBDERE_DPA_2018','08','08110','Talcahuano'),
('SUBDERE_DPA_2018','08','08111','Tomé'),
('SUBDERE_DPA_2018','08','08112','Hualpén'),
('SUBDERE_DPA_2018','08','08201','Lebu'),
('SUBDERE_DPA_2018','08','08202','Arauco'),
('SUBDERE_DPA_2018','08','08203','Cañete'),
('SUBDERE_DPA_2018','08','08204','Contulmo'),
('SUBDERE_DPA_2018','08','08205','Curanilahue'),
('SUBDERE_DPA_2018','08','08206','Los Alamos'),
('SUBDERE_DPA_2018','08','08207','Tirúa'),
('SUBDERE_DPA_2018','08','08301','Los Angeles'),
('SUBDERE_DPA_2018','08','08302','Antuco'),
('SUBDERE_DPA_2018','08','08303','Cabrero'),
('SUBDERE_DPA_2018','08','08304','Laja'),
('SUBDERE_DPA_2018','08','08305','Mulchén'),
('SUBDERE_DPA_2018','08','08306','Nacimiento'),
('SUBDERE_DPA_2018','08','08307','Negrete'),
('SUBDERE_DPA_2018','08','08308','Quilaco'),
('SUBDERE_DPA_2018','08','08309','Quilleco'),
('SUBDERE_DPA_2018','08','08310','San Rosendo'),
('SUBDERE_DPA_2018','08','08311','Santa Bárbara'),
('SUBDERE_DPA_2018','08','08312','Tucapel'),
('SUBDERE_DPA_2018','08','08313','Yumbel'),
('SUBDERE_DPA_2018','08','08314','Alto Biobío'),
('SUBDERE_DPA_2018','09','09101','Temuco'),
('SUBDERE_DPA_2018','09','09102','Carahue'),
('SUBDERE_DPA_2018','09','09103','Cunco'),
('SUBDERE_DPA_2018','09','09104','Curarrehue'),
('SUBDERE_DPA_2018','09','09105','Freire'),
('SUBDERE_DPA_2018','09','09106','Galvarino'),
('SUBDERE_DPA_2018','09','09107','Gorbea'),
('SUBDERE_DPA_2018','09','09108','Lautaro'),
('SUBDERE_DPA_2018','09','09109','Loncoche'),
('SUBDERE_DPA_2018','09','09110','Melipeuco'),
('SUBDERE_DPA_2018','09','09111','Nueva Imperial'),
('SUBDERE_DPA_2018','09','09112','Padre Las Casas'),
('SUBDERE_DPA_2018','09','09113','Perquenco'),
('SUBDERE_DPA_2018','09','09114','Pitrufquén'),
('SUBDERE_DPA_2018','09','09115','Pucón'),
('SUBDERE_DPA_2018','09','09116','Saavedra'),
('SUBDERE_DPA_2018','09','09117','Teodoro Schmidt'),
('SUBDERE_DPA_2018','09','09118','Toltén'),
('SUBDERE_DPA_2018','09','09119','Vilcún'),
('SUBDERE_DPA_2018','09','09120','Villarrica'),
('SUBDERE_DPA_2018','09','09121','Cholchol'),
('SUBDERE_DPA_2018','09','09201','Angol'),
('SUBDERE_DPA_2018','09','09202','Collipulli'),
('SUBDERE_DPA_2018','09','09203','Curacautín'),
('SUBDERE_DPA_2018','09','09204','Ercilla'),
('SUBDERE_DPA_2018','09','09205','Lonquimay'),
('SUBDERE_DPA_2018','09','09206','Los Sauces'),
('SUBDERE_DPA_2018','09','09207','Lumaco'),
('SUBDERE_DPA_2018','09','09208','Purén'),
('SUBDERE_DPA_2018','09','09209','Renaico'),
('SUBDERE_DPA_2018','09','09210','Traiguén'),
('SUBDERE_DPA_2018','09','09211','Victoria'),
('SUBDERE_DPA_2018','10','10101','Puerto Montt'),
('SUBDERE_DPA_2018','10','10102','Calbuco'),
('SUBDERE_DPA_2018','10','10103','Cochamó'),
('SUBDERE_DPA_2018','10','10104','Fresia'),
('SUBDERE_DPA_2018','10','10105','Frutillar'),
('SUBDERE_DPA_2018','10','10106','Los Muermos'),
('SUBDERE_DPA_2018','10','10107','Llanquihue'),
('SUBDERE_DPA_2018','10','10108','Maullín'),
('SUBDERE_DPA_2018','10','10109','Puerto Varas'),
('SUBDERE_DPA_2018','10','10201','Castro'),
('SUBDERE_DPA_2018','10','10202','Ancud'),
('SUBDERE_DPA_2018','10','10203','Chonchi'),
('SUBDERE_DPA_2018','10','10204','Curaco de Vélez'),
('SUBDERE_DPA_2018','10','10205','Dalcahue'),
('SUBDERE_DPA_2018','10','10206','Puqueldón'),
('SUBDERE_DPA_2018','10','10207','Queilén'),
('SUBDERE_DPA_2018','10','10208','Quellón'),
('SUBDERE_DPA_2018','10','10209','Quemchi'),
('SUBDERE_DPA_2018','10','10210','Quinchao'),
('SUBDERE_DPA_2018','10','10301','Osorno'),
('SUBDERE_DPA_2018','10','10302','Puerto Octay'),
('SUBDERE_DPA_2018','10','10303','Purranque'),
('SUBDERE_DPA_2018','10','10304','Puyehue'),
('SUBDERE_DPA_2018','10','10305','Río Negro'),
('SUBDERE_DPA_2018','10','10306','San Juan de la Costa'),
('SUBDERE_DPA_2018','10','10307','San Pablo'),
('SUBDERE_DPA_2018','10','10401','Chaitén'),
('SUBDERE_DPA_2018','10','10402','Futaleufú'),
('SUBDERE_DPA_2018','10','10403','Hualaihué'),
('SUBDERE_DPA_2018','10','10404','Palena'),
('SUBDERE_DPA_2018','11','11101','Coihaique'),
('SUBDERE_DPA_2018','11','11102','Lago Verde'),
('SUBDERE_DPA_2018','11','11201','Aisén'),
('SUBDERE_DPA_2018','11','11202','Cisnes'),
('SUBDERE_DPA_2018','11','11203','Guaitecas'),
('SUBDERE_DPA_2018','11','11301','Cochrane'),
('SUBDERE_DPA_2018','11','11302','O''Higgins'),
('SUBDERE_DPA_2018','11','11303','Tortel'),
('SUBDERE_DPA_2018','11','11401','Chile Chico'),
('SUBDERE_DPA_2018','11','11402','Río Ibáñez'),
('SUBDERE_DPA_2018','12','12101','Punta Arenas'),
('SUBDERE_DPA_2018','12','12102','Laguna Blanca'),
('SUBDERE_DPA_2018','12','12103','Río Verde'),
('SUBDERE_DPA_2018','12','12104','San Gregorio'),
('SUBDERE_DPA_2018','12','12201','Cabo de Hornos'),
('SUBDERE_DPA_2018','12','12301','Porvenir'),
('SUBDERE_DPA_2018','12','12302','Primavera'),
('SUBDERE_DPA_2018','12','12303','Timaukel'),
('SUBDERE_DPA_2018','12','12401','Natales'),
('SUBDERE_DPA_2018','12','12402','Torres del Paine'),
('SUBDERE_DPA_2018','13','13101','Santiago'),
('SUBDERE_DPA_2018','13','13102','Cerrillos'),
('SUBDERE_DPA_2018','13','13103','Cerro Navia'),
('SUBDERE_DPA_2018','13','13104','Conchalí'),
('SUBDERE_DPA_2018','13','13105','El Bosque'),
('SUBDERE_DPA_2018','13','13106','Estación Central'),
('SUBDERE_DPA_2018','13','13107','Huechuraba'),
('SUBDERE_DPA_2018','13','13108','Independencia'),
('SUBDERE_DPA_2018','13','13109','La Cisterna'),
('SUBDERE_DPA_2018','13','13110','La Florida'),
('SUBDERE_DPA_2018','13','13111','La Granja'),
('SUBDERE_DPA_2018','13','13112','La Pintana'),
('SUBDERE_DPA_2018','13','13113','La Reina'),
('SUBDERE_DPA_2018','13','13114','Las Condes'),
('SUBDERE_DPA_2018','13','13115','Lo Barnechea'),
('SUBDERE_DPA_2018','13','13116','Lo Espejo'),
('SUBDERE_DPA_2018','13','13117','Lo Prado'),
('SUBDERE_DPA_2018','13','13118','Macul'),
('SUBDERE_DPA_2018','13','13119','Maipú'),
('SUBDERE_DPA_2018','13','13120','Ñuñoa'),
('SUBDERE_DPA_2018','13','13121','Pedro Aguirre Cerda'),
('SUBDERE_DPA_2018','13','13122','Peñalolén'),
('SUBDERE_DPA_2018','13','13123','Providencia'),
('SUBDERE_DPA_2018','13','13124','Pudahuel'),
('SUBDERE_DPA_2018','13','13125','Quilicura'),
('SUBDERE_DPA_2018','13','13126','Quinta Normal'),
('SUBDERE_DPA_2018','13','13127','Recoleta'),
('SUBDERE_DPA_2018','13','13128','Renca'),
('SUBDERE_DPA_2018','13','13129','San Joaquín'),
('SUBDERE_DPA_2018','13','13130','San Miguel'),
('SUBDERE_DPA_2018','13','13131','San Ramón'),
('SUBDERE_DPA_2018','13','13132','Vitacura'),
('SUBDERE_DPA_2018','13','13201','Puente Alto'),
('SUBDERE_DPA_2018','13','13202','Pirque'),
('SUBDERE_DPA_2018','13','13203','San José de Maipo'),
('SUBDERE_DPA_2018','13','13301','Colina'),
('SUBDERE_DPA_2018','13','13302','Lampa'),
('SUBDERE_DPA_2018','13','13303','Tiltil'),
('SUBDERE_DPA_2018','13','13401','San Bernardo'),
('SUBDERE_DPA_2018','13','13402','Buin'),
('SUBDERE_DPA_2018','13','13403','Calera de Tango'),
('SUBDERE_DPA_2018','13','13404','Paine'),
('SUBDERE_DPA_2018','13','13501','Melipilla'),
('SUBDERE_DPA_2018','13','13502','Alhué'),
('SUBDERE_DPA_2018','13','13503','Curacaví'),
('SUBDERE_DPA_2018','13','13504','María Pinto'),
('SUBDERE_DPA_2018','13','13505','San Pedro'),
('SUBDERE_DPA_2018','13','13601','Talagante'),
('SUBDERE_DPA_2018','13','13602','El Monte'),
('SUBDERE_DPA_2018','13','13603','Isla de Maipo'),
('SUBDERE_DPA_2018','13','13604','Padre Hurtado'),
('SUBDERE_DPA_2018','13','13605','Peñaflor'),
('SUBDERE_DPA_2018','14','14101','Valdivia'),
('SUBDERE_DPA_2018','14','14102','Corral'),
('SUBDERE_DPA_2018','14','14103','Lanco'),
('SUBDERE_DPA_2018','14','14104','Los Lagos'),
('SUBDERE_DPA_2018','14','14105','Máfil'),
('SUBDERE_DPA_2018','14','14106','Mariquina'),
('SUBDERE_DPA_2018','14','14107','Paillaco'),
('SUBDERE_DPA_2018','14','14108','Panguipulli'),
('SUBDERE_DPA_2018','14','14201','La Unión'),
('SUBDERE_DPA_2018','14','14202','Futrono'),
('SUBDERE_DPA_2018','14','14203','Lago Ranco'),
('SUBDERE_DPA_2018','14','14204','Río Bueno'),
('SUBDERE_DPA_2018','15','15101','Arica'),
('SUBDERE_DPA_2018','15','15102','Camarones'),
('SUBDERE_DPA_2018','15','15201','Putre'),
('SUBDERE_DPA_2018','15','15202','General Lagos'),
('SUBDERE_DPA_2018','16','16101','Chillán'),
('SUBDERE_DPA_2018','16','16102','Bulnes'),
('SUBDERE_DPA_2018','16','16103','Chillán Viejo'),
('SUBDERE_DPA_2018','16','16104','El Carmen'),
('SUBDERE_DPA_2018','16','16105','Pemuco'),
('SUBDERE_DPA_2018','16','16106','Pinto'),
('SUBDERE_DPA_2018','16','16107','Quillón'),
('SUBDERE_DPA_2018','16','16108','San Ignacio'),
('SUBDERE_DPA_2018','16','16109','Yungay'),
('SUBDERE_DPA_2018','16','16201','Quirihue'),
('SUBDERE_DPA_2018','16','16202','Cobquecura'),
('SUBDERE_DPA_2018','16','16203','Coelemu'),
('SUBDERE_DPA_2018','16','16204','Ninhue'),
('SUBDERE_DPA_2018','16','16205','Portezuelo'),
('SUBDERE_DPA_2018','16','16206','Ranquil'),
('SUBDERE_DPA_2018','16','16207','Treguaco'),
('SUBDERE_DPA_2018','16','16301','San Carlos'),
('SUBDERE_DPA_2018','16','16302','Coihueco'),
('SUBDERE_DPA_2018','16','16303','Ñiquén'),
('SUBDERE_DPA_2018','16','16304','San Fabián'),
('SUBDERE_DPA_2018','16','16305','San Nicolás');

ALTER TABLE public.espacio
    ADD COLUMN ubicacion_catalogo_version text,
    ADD COLUMN region_codigo text,
    ADD COLUMN comuna_codigo text,
    ADD COLUMN busqueda_publica tsvector NOT NULL DEFAULT ''::tsvector,
    ADD CONSTRAINT espacio_public_location_all_or_none_ck CHECK (
        (ubicacion_catalogo_version IS NULL AND region_codigo IS NULL AND comuna_codigo IS NULL)
        OR (ubicacion_catalogo_version IS NOT NULL AND region_codigo IS NOT NULL AND comuna_codigo IS NOT NULL)
    ),
    ADD CONSTRAINT espacio_public_region_fk
        FOREIGN KEY (ubicacion_catalogo_version, region_codigo)
        REFERENCES public.region_publica(catalogo_version, codigo),
    ADD CONSTRAINT espacio_public_commune_fk
        FOREIGN KEY (ubicacion_catalogo_version, region_codigo, comuna_codigo)
        REFERENCES public.comuna_publica(catalogo_version, region_codigo, codigo);

CREATE INDEX espacio_public_region_idx
    ON public.espacio(ubicacion_catalogo_version,region_codigo,comuna_codigo,id)
    WHERE estado='activa' AND ubicacion_catalogo_version IS NOT NULL;
CREATE INDEX espacio_public_area_idx
    ON public.espacio(superficie_m2,id)
    WHERE estado='activa';
CREATE INDEX espacio_public_search_gin_idx
    ON public.espacio USING gin(busqueda_publica)
    WHERE estado='activa';
CREATE INDEX tarifa_espacio_latest_idx
    ON public.tarifa_espacio(espacio_id,version DESC);
CREATE INDEX espacio_caracteristicas_public_values_gin_idx
    ON public.espacio_caracteristicas USING gin(valores jsonb_path_ops);

-- The reviewed allowlist is explicit and profile-versioned. It deliberately
-- selects only typed boolean amenities from V6; other profile values remain
-- unavailable to public search even if present in the JSON object.
CREATE TABLE public.atributo_filtro_publico (
    categoria_codigo text NOT NULL,
    perfil_version integer NOT NULL CHECK (perfil_version > 0),
    codigo text NOT NULL CHECK (codigo ~ '^[a-z][a-z0-9_]{0,59}$'),
    etiqueta text NOT NULL CHECK (length(btrim(etiqueta)) BETWEEN 1 AND 80),
    tipo text NOT NULL CHECK (tipo IN ('boolean')),
    orden smallint NOT NULL CHECK (orden > 0),
    PRIMARY KEY (categoria_codigo,perfil_version,codigo),
    UNIQUE (categoria_codigo,perfil_version,orden),
    FOREIGN KEY (categoria_codigo,perfil_version)
        REFERENCES public.categoria_perfil_atributos(categoria_codigo,version)
);
INSERT INTO public.atributo_filtro_publico(categoria_codigo,perfil_version,codigo,etiqueta,tipo,orden) VALUES
('oficina',1,'wifi','Wi-Fi','boolean',1),
('oficina',1,'mobiliario_incluido','Mobiliario incluido','boolean',2),
('sala_multiproposito',1,'wifi','Wi-Fi','boolean',1),
('sala_multiproposito',1,'proyector','Proyector','boolean',2),
('bodega',1,'carro_carga_disponible','Carro de carga','boolean',1),
('bodega',1,'muelle_carga','Muelle de carga','boolean',2),
('estacionamiento',1,'techado','Techado','boolean',1),
('estacionamiento',1,'carga_electrica','Carga eléctrica','boolean',2),
('local_flexible',1,'acceso_nivel_calle','Acceso a nivel de calle','boolean',1),
('local_flexible',1,'vitrina','Vitrina','boolean',2),
('stand',1,'montaje_incluido','Montaje incluido','boolean',1),
('stand',1,'wifi','Wi-Fi','boolean',2),
('quincho',1,'techado','Techado','boolean',1),
('quincho',1,'agua_potable','Agua potable','boolean',2),
('parcela_eventos',1,'salon_cubierto','Salón cubierto','boolean',1),
('parcela_eventos',1,'quincho_disponible','Quincho disponible','boolean',2);

CREATE FUNCTION public.build_public_search_document(
    p_status text, p_category text, p_title text, p_description text,
    p_location_version text, p_region_code text, p_commune_code text
) RETURNS tsvector
LANGUAGE plpgsql STABLE PARALLEL SAFE AS $$
DECLARE
    category_name text;
    region_name text;
    commune_name text;
    text_value text;
BEGIN
    IF p_status <> 'activa' THEN RETURN ''::tsvector; END IF;
    SELECT c.nombre INTO category_name FROM public.categoria_espacio c WHERE c.codigo=p_category;
    IF p_location_version IS NOT NULL AND p_region_code IS NOT NULL AND p_commune_code IS NOT NULL THEN
        SELECT r.nombre, m.nombre INTO region_name, commune_name
        FROM public.region_publica r
        JOIN public.comuna_publica m
          ON m.catalogo_version=r.catalogo_version AND m.region_codigo=r.codigo
        WHERE r.catalogo_version=p_location_version
          AND r.codigo=p_region_code AND m.codigo=p_commune_code;
    END IF;
    text_value := normalize(regexp_replace(concat_ws(' ',p_title,p_description,category_name,p_category,region_name,commune_name), '[[:space:]]+', ' ', 'g'), NFC);
    RETURN to_tsvector('public.espacigo_es_unaccent'::regconfig, text_value);
END;
$$;

CREATE FUNCTION public.refresh_espacio_public_search_document() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    NEW.busqueda_publica := public.build_public_search_document(
        NEW.estado,NEW.categoria_codigo,NEW.titulo,NEW.descripcion,
        NEW.ubicacion_catalogo_version,NEW.region_codigo,NEW.comuna_codigo
    );
    RETURN NEW;
END;
$$;
CREATE TRIGGER espacio_public_search_document_trg
    BEFORE INSERT OR UPDATE OF estado,categoria_codigo,titulo,descripcion,
        ubicacion_catalogo_version,region_codigo,comuna_codigo
    ON public.espacio
    FOR EACH ROW EXECUTE FUNCTION public.refresh_espacio_public_search_document();

CREATE FUNCTION public.refresh_category_public_search_document() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE public.espacio SET descripcion=descripcion WHERE categoria_codigo=NEW.codigo;
    RETURN NEW;
END;
$$;
CREATE TRIGGER categoria_public_search_document_trg
    AFTER UPDATE OF nombre ON public.categoria_espacio
    FOR EACH ROW WHEN (OLD.nombre IS DISTINCT FROM NEW.nombre)
    EXECUTE FUNCTION public.refresh_category_public_search_document();

-- Backfill only the search vector; location columns remain NULL for every
-- existing offer until its host supplies a public zone in MPV2-03.
UPDATE public.espacio e SET busqueda_publica = public.build_public_search_document(
    e.estado,e.categoria_codigo,e.titulo,e.descripcion,
    e.ubicacion_catalogo_version,e.region_codigo,e.comuna_codigo
) WHERE e.estado='activa';

-- This SQL key matches booking.PriceUnits. Go recomputes the page's displayed
-- exact CLP totals with PriceUnits; SQL uses the same units before LIMIT/sort.
CREATE FUNCTION public.public_catalog_billed_units(
    p_unit text,p_start timestamptz,p_end timestamptz,p_zone text
) RETURNS bigint
LANGUAGE plpgsql STABLE STRICT PARALLEL SAFE AS $$
DECLARE
    local_start timestamp;
    local_end timestamp;
    month_delta integer;
    target_month date;
    anniversary_day integer;
    target_day integer;
    anniversary_local timestamp;
BEGIN
    IF p_end <= p_start OR p_zone = '' THEN RETURN NULL; END IF;
    local_start := p_start AT TIME ZONE p_zone;
    local_end := p_end AT TIME ZONE p_zone;
    CASE p_unit
    WHEN 'hora' THEN RETURN GREATEST(1,ceil(extract(epoch FROM (p_end-p_start))/3600.0)::bigint);
    WHEN 'dia' THEN
        target_day := extract(day FROM local_end)::integer;
        IF local_end::time = TIME '00:00:00' THEN target_day := target_day-1; END IF;
        RETURN (make_date(extract(year FROM local_end)::integer,extract(month FROM local_end)::integer,1)
                + (target_day-1))
             - local_start::date + 1;
    WHEN 'mes' THEN
        month_delta := (extract(year FROM local_end)::integer-extract(year FROM local_start)::integer)*12
                     + extract(month FROM local_end)::integer-extract(month FROM local_start)::integer;
        IF month_delta < 1 THEN RETURN 1; END IF;
        target_month := (date_trunc('month',local_start)::date + make_interval(months=>month_delta))::date;
        anniversary_day := extract(day FROM local_start)::integer;
        target_day := LEAST(anniversary_day,extract(day FROM (target_month + interval '1 month - 1 day'))::integer);
        anniversary_local := target_month + (target_day-1) + local_start::time;
        IF local_end > (anniversary_local AT TIME ZONE p_zone) THEN month_delta := month_delta+1; END IF;
        RETURN month_delta;
    ELSE RETURN NULL;
    END CASE;
EXCEPTION WHEN invalid_parameter_value OR datetime_field_overflow THEN
    RETURN NULL;
END;
$$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN
        EXECUTE 'GRANT SELECT ON TABLE public.catalogo_ubicacion_publica, public.region_publica, public.comuna_publica, public.atributo_filtro_publico TO espacigo_runtime';
    END IF;
END;
$$;
