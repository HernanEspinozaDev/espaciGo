# Contrato de implementación DEMO-01 — solo memoria y loopback

No se monta en cmd/api ni usa SQL, booking M06, KYC, contratos, pagos o cuentas reales. Ejecutable propio `cmd/rentaldemo`, listener numérico loopback obligatorio. El frontend solo habilita rutas demo con RENTAL_DEMO_ENABLED=1 y RENTAL_DEMO_BASE_URL validada loopback; inaccesibles por defecto. Cookie HttpOnly demo independiente de la sesión de producto; nunca usar el token de producto. El estado se pierde al reiniciar Go y ambos calendarios usan ese mismo proceso.

## HTTP

Prefijo Go `/api/v1/local/rental-demo`. BFF Next `/api/rental-demo`. JSON directo (sin envelope data); errores genéricos `{error:{code}}`, no-store siempre. Request JSON estricto/bounded, consultas sin duplicados/extras. Mutaciones BFF requieren Origin igual al Host loopback. El BFF no reenvía cookies de producto ni Authorization, no sigue redirecciones y solo permite rutas demo enumeradas.

- GET /session → `{actor: "host"|"renter"|"renter2", expires_at:string}` o 401.
- POST /session `{actor:...}` → mismo DTO y cookie `espacigo_rental_demo`, HttpOnly SameSite Strict, Path=/api/rental-demo. Son actores sintéticos seleccionables, no cuentas ni autenticación comercial.
- DELETE /session → 204 y cookie retirada.
- GET /spaces → `Space[]`, solo activas.
- GET /owner/spaces → `Space[]`, requiere host; incluye borrador/oculta.
- GET /spaces/{id}/month?month=YYYY-MM&duration=N → Month, solo activas; sin login.
- GET /owner/spaces/{id}/month?month=YYYY-MM&duration=N → Month + blocks, requiere host.
- PUT /owner/spaces/{id} `{expected_revision:string,state:"activa"|"borrador"|"oculta",unit_price_clp:string,open_hour:number,close_hour:number}` → Space, host, CAS; open/close enteros 0..24 ordenados; afecta solo recurrencia horaria demo.
- POST /owner/spaces/{id}/blocks `{expected_revision:string,start_at:string,end_at:string}` → `{id:string}`, host, CAS.
- DELETE /owner/spaces/{id}/blocks/{blockId}, If-Match: revision decimal entre comillas → 204, host CAS. No borra ocupaciones.
- POST /quotes `{space_id:string,start_at:string,end_at:string}` → Quote, renter/renter2; sin retención, revalida publicación/recurrencia/bloqueos/ocupación; cotización demo con TTL 10 minutos (valor de ensayo, no política comercial).
- POST /quotes/{quoteId}/confirm `{approved_contract:true,intent_id:uuid}` → Confirmation. Requiere mismo renter; revalida snapshot/disponibilidad y ocupa en memoria de forma atómica. Misma intención/mismo quote devuelve mismo resultado sin duplicar; intención reutilizada para otro quote →409. No KYC/contrato/proveedor. Casilla false/ausente no confirma.
- GET /attempts/{intentId} → Confirmation del mismo actor, o404. Consulta tras resultado incierto, nunca replay automático.

## DTO compartidos

```typescript
type Interval = {start_at:string;end_at:string};
type Space = {id:string;title:string;state:"activa"|"borrador"|"oculta";time_zone:string;rate_unit:"hora"|"dia"|"mes";unit_price_clp:string;revision:string;open_hour:number;close_hour:number};
type Day = {date:string;status:"available"|"partial"|"unavailable"|"outside_horizon";intervals:Interval[]};
type Month = {space_id:string;title:string;time_zone:string;rate_unit:"hora"|"dia"|"mes";unit_price_clp:string;revision:string;month:string;today:string;days:Day[];blocks?:({id:string}&Interval)[]};
type Quote = {id:string;space_id:string;space_title:string;start_at:string;end_at:string;time_zone:string;rate_unit:"hora"|"dia"|"mes";unit_price_clp:string;units:number;subtotal_clp:string;currency:"CLP";expires_at:string};
type Confirmation = Quote & {quote_id:string;intent_id:string;status:"confirmed_demo";payment_status:"paid_simulated"};
```

Public Month n'expose ni propriétaire, blocs, motif, occupant, ID réserve, révision interne commerciale ou identité. `revision` est uniquement le compteur synthétique de cette démo et doit être omis du DTO public: **Month.revision est uniquement présent en mode owner**. Space public conserve uniquement métadonnées synthétiques de la fixture. Toute donnée est synthétique; pas de transposition à un compte réel.

## Règles

Grille heure :00/:30 locale, durées 1/2/4 heures réellement écoulées. Jour 1..3 à partir du premier instant de date locale, mois 1 anniversaire clamped. Horizon date de début aujourd'hui..aujourd'hui+90 inclus, début strictement futur. Slots half-open; rejeter dates omises/malformed; DST ambigu conserver les instants distincts et afficher offset/zone. Le mois renvoie toutes ses dates avec outside_horizon explicit; aucune disponibilité par défaut. Statut partial si seuls certains candidats de la journée survivent aux blocs/occupations ou si une journée horaire n'est ouverte qu'en partie. Calcul CLP exact côté Go; strings au wire. Quote snapshot conserve révision interne pour revalidation, sans la sérialiser. Révision change sur blocage, configuration et confirmation; aucun changement sur GET ou quote. Les confirmations restent liées au sujet/intention/quote/snapshot, même après changement de tarif.

## UI et preuve

Composant mensuel partagé, vraie grille, mois précédent/suivant/Hoy, jour choisi, légende textuelle, panneau d'intervalles et zone. Owner dans gestion d'espace demo et public dans fiche demo accessibles par liens; ne pas modifier la publication online. Casilla «Aprobar contrato» non cochée par défaut et réinitialisée à tout changement de session/espace/intervalle/cotisation. «Confirmación simulada: no se genera ni firma un contrato», «KYC omitido en esta demo» et «Pagado (simulado)» permanents pertinents. Nav public → accès demo → même espace, compte synthétique; autorisations déterminées en Go. Toujours vérifier une seule source Go via UI host, UI public et readback après confirmation. Routes demo indisponibles sans configuration explicite. Aucun compte réel, secret, DB ou appel fournisseur.
