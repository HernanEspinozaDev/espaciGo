# Matriz de paridad del mock hacia Angular

Fecha: 2026-10-13. Snapshot OpenAPI 3.1.0 de `b0f62b976c9586d260cea15ed9acc29ab12257a7`: **118 paths / 144 operaciones**. Inventario de contrato; la evidencia y el estado FE-02 se registran abajo. El estado de cada operación debe leerse junto a la traza y límites del corte; la presencia en inventario no implica una repetición E2E en FE-14.

## Cómo usarla

- Registrar por operación: pantalla/componente, rol/capacidad, prueba enfocada/E2E, commit/PR y evidencia. Los estados Pendiente indican trabajo no cubierto por el corte actualmente en revisión.
- La seguridad indicada aquí es la declaración OpenAPI. Go añade autorización por rol, recurso, elegibilidad y modo restringido; verificarla en cada corte. Bearer no significa que cualquier cuenta tenga permiso.
- Endpoints técnicos/compatibilidad se justifican sin una pantalla directa. No retirar la API por retirar el mock. Los healthchecks y comandos de workers/replay/migración quedan en infraestructura.
- El inventario no obliga a un botón por endpoint. Una pantalla puede coordinar varias operaciones y un endpoint puede servir a varias pantallas.
- Si aparece una operación del mock/router no descrita por OpenAPI, registrar esa diferencia en FE-00 y resolverla antes de declarar paridad; estos conteos no certifican cobertura del router completo.
- Las respuestas PDF/PNG/ZIP requieren acceso autenticado, manejo de Blob y pruebas de sesión/selección. No publicar una URL privada en un img/iframe sin transporte autorizado.

## Distribución por corte

| Corte | Operaciones declaradas | Estado Angular |
| --- | ---: | --- |
| Compatibilidad | 1 | No requiere pantalla directa |
| FE-02 | 10 | Aprobado y fusionado; PR #275 |
| FE-03 | 7 | Aceptado y fusionado; PR #276 |
| FE-04 | 13 | Aceptado y fusionado; PR #278 |
| FE-05 | 10 | Aceptado y fusionado; PR #279 |
| FE-06 | 26 | Aceptado y fusionado; PR #280 |
| FE-07 | 8 | Aceptado y fusionado; PR #282; merge `978a27e` |
| FE-08 | 10 | Aprobado y fusionado; [PR #283](https://github.com/HernanEspinozaDev/espaciGo/pull/283), merge `503d193` |
| FE-09 | 8 | Aceptado y fusionado; PR #284, merge `0d3bac8` |
| FE-10 | 21 | Aceptado y fusionado en [PR #285](https://github.com/HernanEspinozaDev/espaciGo/pull/285), merge `39514662c9002d3c50bdbe7244813c57d13c2b38` |
| FE-11 | 29 | Aceptado y fusionado; [PR #286](https://github.com/HernanEspinozaDev/espaciGo/pull/286) |
| FE-12 | 0 | Aceptado y fusionado; [PR #287](https://github.com/HernanEspinozaDev/espaciGo/pull/287) |
| FE-13 | 0 | Aceptado y fusionado en [PR #288](https://github.com/HernanEspinozaDev/espaciGo/pull/288); stack Angular Docker, URL/CORS y runner independiente del mock |
| FE-14 | 0 | Regresión consolidada/aceptación; PR de este corte |
| Técnica | 1 | No requiere pantalla directa |

La cobertura completa requiere FE-01/12–15 para diseño, Docker, calidad y retirada, aunque no agreguen operaciones de API.

## Inventario por operación

| Método y ruta relativa a `/api/v1` | operationId | Acceso declarado | Corte / pantalla | Estado |
| --- | --- | --- | --- | --- |
| `GET /spaces/{spaceID}/reviews` | `listActiveSpaceReviews` | Sin Bearer declarado; reglas en Go | FE-09 — Reseñas visibles autorizadas | Implementado; espacio activo Go HTTP 200, 1 reseña |
| `GET /local/booking-trial/reservations/{reservationID}/reviews` | `listReservationReviews` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Implementado; stack Go + regresión Angular |
| `POST /local/booking-trial/reservations/{reservationID}/reviews` | `createReservationReview` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Implementado; anfitrión y arrendatario |
| `POST /local/booking-trial/reservations/{reservationID}/reviews/{reviewID}/report` | `reportSpaceReview` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Implementado; reporte host confirmado |
| `GET /local/reputation/me` | `getOwnRenterReputation` | Bearer; permisos en Go | FE-09 — Reputación propia | Implementado; stack Go |
| `GET /admin/local/review-reports` | `listPendingReviewReports` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `POST /admin/local/review-reports/{reportID}/{action}` | `moderateReviewReport` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/local/notices` | `listTerminalLocalNotices` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `POST /admin/local/notices/{noticeID}/reopen` | `reopenLocalNotice` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /auth/terms` | `listLocalTerms` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/register` | `registerAccount` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/verification/reissue` | `reissueVerification` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/verification` | `verifyEmail` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/password/recovery` | `requestPasswordRecovery` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/password/recovery/consume` | `consumePasswordRecovery` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/password/change` | `changePassword` | Bearer; permisos en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `GET /profile` | `getOwnProfile` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; aceptado y fusionado en PR #278 |
| `PUT /profile` | `updateOwnProfile` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; aceptado y fusionado en PR #278 |
| `GET /profile/photo` | `getOwnSyntheticProfilePhoto` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; aceptado y fusionado en PR #278 |
| `PUT /profile/photo` | `generateOwnSyntheticProfilePhoto` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; aceptado y fusionado en PR #278 |
| `DELETE /profile/photo` | `removeOwnSyntheticProfilePhoto` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; aceptado y fusionado en PR #278 |
| `GET /profile/photo/content` | `getOwnSyntheticProfilePhotoContent` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; aceptado y fusionado en PR #278 |
| `GET /payout-account` | `getOwnSyntheticPayoutAccount` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; aceptado y fusionado en PR #278 |
| `PUT /payout-account` | `createOrReplaceOwnSyntheticPayoutAccount` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; aceptado y fusionado en PR #278 |
| `DELETE /payout-account` | `revokeOwnSyntheticPayoutAccount` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; aceptado y fusionado en PR #278 |
| `GET /rights-requests` | `listOwnRightsRequests` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; aceptado y fusionado en PR #278 |
| `POST /rights-requests` | `requestDataRight` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; aceptado y fusionado en PR #278 |
| `GET /privacy/export` | `exportOwnIdentityData` | Bearer; permisos en Go | FE-04 — Exportación del titular | Implementado; aceptado y fusionado en PR #278 |
| `GET /privacy/export/archive` | `downloadCompleteOwnDataArchive` | Bearer; permisos en Go | FE-04 — Exportación del titular | Implementado; aceptado y fusionado en PR #278 |
| `POST /privacy/suppression-requests/{request_id}/review` | `reviewLocalSuppressionBlockers` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `POST /privacy/suppression-requests/{request_id}/execute` | `executeLocalSyntheticSuppression` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `POST /privacy/retention/purge` | `purgeExpiredLocalPrivacyData` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/credential-notices` | `listTerminalLocalCredentialNotices` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `POST /admin/credential-notices/{event_id}/reopen` | `reopenTerminalLocalCredentialNotice` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /privacy/suppression-requests` | `listPendingLocalSuppressionRequests` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /verifications` | `listOwnVerifications` | Bearer; permiso titular en Go | FE-05 — Casos KYC/KYB propios | Implementado |
| `POST /verifications` | `startSyntheticVerification` | Bearer; permiso e idempotencia Go | FE-05 — Crear caso sintético | Implementado; sólo KYC/KYB |
| `GET /verifications/eligibility` | `listOwnSyntheticEligibility` | Bearer; elegibilidad efectiva por Go | FE-05 — Elegibilidad por tipo | Implementado; no muta roles |
| `GET /verifications/{id}` | `getOwnVerification` | Bearer; permiso de titular en Go | FE-05 — Detalle | Implementado; respuestas tardías aisladas |
| `GET /verifications/{id}/history` | `listOwnVerificationHistory` | Bearer; permiso de titular en Go | FE-05 — Historial | Implementado |
| `POST /verifications/{id}/retry` | `retryRejectedVerification` | Bearer; rechazo/código/idempotencia validados por Go | FE-05 — Subsanación | Implementado; códigos del contrato |
| `GET /verifications/{id}/evidence` | `listOwnSyntheticEvidence` | Bearer; titular; metadatos | FE-05 — Evidencia propia | Implementado; vacío/retirado |
| `POST /verifications/{id}/evidence` | `uploadSyntheticEvidence` | Bearer; fixture e idempotencia validados por Go | FE-05 — Evidencia | Implementado; sólo `synthetic-png-v1`, sin cargas |
| `GET /verifications/{id}/evidence/{evidenceId}` | `getOwnSyntheticEvidenceContent` | Bearer; autorización titular | FE-05 — PNG privado | Implementado; Blob URL temporal |
| `DELETE /verifications/{id}/evidence/{evidenceId}` | `deleteOwnSyntheticEvidence` | Bearer; permiso titular en Go | FE-05 — Retirar evidencia | Disponible por contrato; no se expone en UI |
| `GET /admin/verifications/{id}/evidence/{evidenceId}` | `reviewSyntheticEvidenceContent` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `DELETE /admin/verifications/{id}/evidence/{evidenceId}` | `deleteReviewedSyntheticEvidence` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/verifications/{id}/evidence` | `listEvidenceForReview` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/verifications` | `listPendingVerifications` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/verifications/failed` | `listRejectedVerifications` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `POST /admin/verifications/{id}/review` | `reviewSyntheticVerification` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `POST /admin/verifications/{id}/revoke` | `revokeSyntheticEligibility` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `POST /auth/login` | `login` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `GET /auth/session` | `currentSession` | Bearer; permisos en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `GET /spaces/categories` | `listSpaceCategories` | Bearer; permisos en Go | FE-03 — Catálogo de categorías y perfiles | Implementado; aceptado y fusionado en PR #276 |
| `GET /spaces/categories/{categoryCode}/attributes` | `getSpaceAttributeProfile` | Bearer; permisos en Go | FE-03 — Catálogo de categorías y perfiles | Implementado; aceptado y fusionado en PR #276 |
| `GET /spaces/categories/{categoryCode}/attributes/{version}` | `getVersionedSpaceAttributeProfile` | Bearer; permisos en Go | FE-03 — Catálogo de categorías y perfiles | Implementado; aceptado y fusionado en PR #276 |
| `GET /spaces` | `listOwnSpaces` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `POST /spaces` | `createOwnSpaceDraft` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `GET /spaces/{spaceId}` | `getOwnSpaceDraft` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `PUT /spaces/{spaceId}` | `updateOwnSpaceDraft` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `DELETE /spaces/{spaceId}` | `deleteOwnPublication` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `PUT /spaces/{spaceId}/publication` | `setOwnSpacePublicationState` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `PUT /spaces/{spaceId}/publication-content` | `updateOwnPublishedSpaceContent` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `GET /spaces/{spaceId}/gallery` | `listOwnSyntheticSpaceGallery` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `POST /spaces/{spaceId}/gallery` | `createSyntheticOwnSpaceGalleryPhoto` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `PUT /spaces/{spaceId}/cover` | `selectOwnSyntheticSpaceCover` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `GET /spaces/{spaceId}/gallery/{photoId}` | `getOwnSyntheticSpaceGalleryPhoto` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `DELETE /spaces/{spaceId}/gallery/{photoId}` | `removeOwnSyntheticSpaceGalleryPhoto` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `GET /spaces/{spaceId}/gallery/{photoId}/content` | `getOwnSyntheticSpaceGalleryPhotoContent` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `PUT /spaces/{spaceId}/availability` | `configureOwnDraftAvailability` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `GET /spaces/{spaceId}/availability` | `queryOwnDraftAvailability` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `GET /spaces/{spaceId}/tariff` | `getOwnSpaceTariff` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `PUT /spaces/{spaceId}/tariff` | `updateOwnSpaceTariff` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `GET /spaces/{spaceId}/tariffs` | `listOwnSpaceTariffVersions` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `POST /spaces/{spaceId}/price-simulations` | `createPrivateOwnDraftPriceSimulation` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `GET /spaces/{spaceId}/price-simulations/{simulationId}` | `getOwnPrivatePriceSimulation` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `GET /spaces/{spaceId}/availability/blocks` | `listOwnDraftManualBlocks` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `POST /spaces/{spaceId}/availability/blocks` | `createOwnDraftManualBlock` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `GET /spaces/{spaceId}/availability/timezone` | `getOwnDraftTimeZone` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `DELETE /spaces/{spaceId}/availability/blocks/{blockId}` | `deleteOwnDraftManualBlock` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Implementado; PR #280 |
| `GET /local/booking-trial/fixture` | `getAuthorizedLocalBookingFixture` | Bearer; permisos en Go | Compatibilidad — Sustituida en UI por catálogo/detalle; conservar API | Sin UI directa justificada |
| `GET /local/booking-trial/catalog` | `searchAuthorizedLocalSyntheticCatalog` | Bearer; permisos en Go | FE-03 — Exploración, detalle y horarios | Implementado; aceptado y fusionado en PR #276 |
| `GET /local/booking-trial/catalog/{spaceId}` | `getAuthorizedLocalSyntheticCatalogDetail` | Bearer; permisos en Go | FE-03 — Exploración, detalle y horarios | Implementado; aceptado y fusionado en PR #276 |
| `GET /local/booking-trial/catalog/{spaceId}/availability-options` | `getAuthorizedLocalFixtureAvailabilityOptions` | Bearer; permisos en Go | FE-03 — Exploración, detalle y horarios | Implementado; aceptado y fusionado en PR #276 |
| `GET /local/booking-trial/catalog/{spaceId}/weekly-hours` | `getOwnSyntheticHourlyWeeklyHours` | Bearer; permisos en Go | FE-06 — Horario semanal propio | Implementado; PR #280 |
| `PUT /local/booking-trial/catalog/{spaceId}/weekly-hours` | `replaceOwnSyntheticHourlyWeeklyHours` | Bearer; permisos en Go | FE-06 — Horario semanal propio | Implementado; PR #280 |
| `POST /local/booking-trial/quotes` | `quoteAuthorizedLocalBooking` | Bearer; permisos en Go | FE-03 — Cotización confirmada por Go | Implementado; aceptado y fusionado en PR #276 |
| `GET /local/booking-trial/reservations` | `listOwnLocalTrialReservations` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `POST /local/booking-trial/reservations` | `requestLocalTrialReservation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `GET /local/booking-trial/reservations/{reservationId}` | `getOwnLocalTrialReservation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `POST /local/booking-trial/reservations/{reservationId}/payment` | `simulateLocalTrialPayment` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `GET /local/booking-trial/reservations/{reservationId}/settlement` | `getOwnLocalSettlement` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 aprobado y fusionado |
| `POST /local/booking-trial/reservations/{reservationId}/settlement` | `requestLocalFakeSettlement` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 aprobado y fusionado |
| `POST /local/booking-trial/reservations/{reservationId}/settlement-reconcile` | `reconcileLocalFakeSettlement` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 aprobado y fusionado |
| `GET /local/booking-trial/reservations/{reservationId}/receipt` | `getOwnLocalSettlementReceipt` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 aprobado y fusionado |
| `GET /local/booking-trial/reservations/{reservationId}/guarantee` | `getOwnSyntheticReservationGuarantee` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 aprobado y fusionado |
| `POST /local/booking-trial/reservations/{reservationId}/guarantee` | `executeLocalSyntheticGuaranteeOperation` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 aprobado y fusionado |
| `POST /local/booking-trial/reservations/{reservationId}/guarantee-reconcile` | `reconcileLocalGuaranteeFakeResult` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 aprobado y fusionado |
| `POST /local/booking-trial/reservations/{reservationId}/financial-decision` | `recordAdminLocalFinancialDecision` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 aprobado y fusionado |
| `POST /local/booking-trial/payment-events` | `receiveAuthenticatedLocalPaymentEvent` | HMAC Backend | Técnica — Callback HMAC; no UI ni secreto en Angular | Sin UI directa justificada |
| `POST /local/booking-trial/reservations/{reservationId}/decision` | `decideLocalTrialReservation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `GET /local/booking-trial/reservations/{reservationId}/cancellation-preview` | `previewLocalTrialReservationCancellation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `POST /local/booking-trial/reservations/{reservationId}/cancel` | `cancelLocalTrialReservation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `POST /local/booking-trial/reservations/{reservationId}/refund` | `simulateLocalTrialRefund` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `GET /local/booking-trial/reservations/{reservationId}/messages` | `listOwnLocalReservationMessages` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Implementado; 38 mensajes, páginas 30 + anteriores y tercero denegado |
| `POST /local/booking-trial/reservations/{reservationId}/messages` | `sendOwnLocalReservationMessage` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Implementado; estado editable y terminal denegado por Go |
| `POST /local/booking-trial/reservations/{reservationId}/messages/read` | `advanceOwnLocalReservationMessageReadCursor` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Implementado; cursor y contador confirmados por Go |
| `GET /local/booking-trial/reservations/{reservation_id}/disputes` | `listOwnReservationTrialDisputes` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 aprobado y fusionado |
| `POST /local/booking-trial/reservations/{reservation_id}/disputes` | `openOwnHostedTrialDispute` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 aprobado y fusionado |
| `GET /local/booking-trial/disputes/{dispute_id}/history` | `getOwnTrialDisputeHistory` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 aprobado y fusionado |
| `GET /admin/disputes` | `listOpenLocalTrialDisputes` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 aprobado y fusionado |
| `POST /admin/disputes/{dispute_id}/close` | `closeLocalTrialDispute` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 aprobado y fusionado |
| `POST /auth/logout` | `logout` | Bearer; permisos en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /local/booking-trial/reservations/{reservationId}/contract` | `createOrGetSyntheticReservationContract` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Implementado; aprobado y fusionado PR #283 |
| `GET /local/booking-trial/contracts/{contractId}` | `getOwnSyntheticContract` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Implementado; aprobado y fusionado PR #283 |
| `POST /local/booking-trial/contracts/{contractId}/sign` | `signSyntheticContractAsParticipant` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Implementado; aprobado y fusionado PR #283 |
| `POST /local/booking-trial/contracts/{contractId}/reject` | `rejectSyntheticContractVersion` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Implementado; aprobado y fusionado PR #283 |
| `GET /local/booking-trial/contracts/{contractId}/document` | `downloadSignedSyntheticContract` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Implementado; aprobado y fusionado PR #283 |
| `POST /local/booking-trial/reservations/{reservationID}/check-in` | `recordSyntheticCheckIn` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Implementado; aprobado y fusionado PR #283 |
| `POST /local/booking-trial/reservations/{reservationID}/check-out` | `recordSyntheticCheckOut` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Implementado; aprobado y fusionado PR #283 |
| `POST /local/booking-trial/reservations/{reservationID}/reception` | `recordSyntheticHostReception` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Implementado; aprobado y fusionado PR #283 |
| `GET /local/booking-trial/reservations/{reservationID}/operations` | `listSyntheticRentalOperations` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Implementado; aprobado y fusionado PR #283 |
| `GET /local/booking-trial/reservations/{reservationID}/evidence/{evidenceID}` | `downloadSyntheticRentalOperationEvidence` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Implementado; aprobado y fusionado PR #283 |
| `POST /local/booking-trial/reservations/{reservationID}/damage-claim/defense/evidence` | `createSyntheticDamageClaimDefenseEvidence` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 aprobado y fusionado |
| `GET /local/booking-trial/reservations/{reservationID}/damage-claim` | `getOwnSyntheticDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 aprobado y fusionado |
| `POST /local/booking-trial/reservations/{reservationID}/damage-claim` | `openSyntheticHostDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 aprobado y fusionado |
| `POST /local/booking-trial/reservations/{reservationID}/damage-claim/defense` | `submitSyntheticDamageClaimDefense` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 aprobado y fusionado |
| `GET /admin/local/audit-events` | `listAdminLocalAuditEvents` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/local/audit-events/export` | `exportAdminLocalAuditEvents` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/local/accounts/{accountID}` | `getAdminLocalAccountByID` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/local/published-spaces` | `listAdminLocalPublishedSpaces` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/local/published-spaces/{spaceID}` | `getAdminLocalPublishedSpace` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `POST /admin/local/accounts/{accountID}/block` | `blockLocalAccount` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `POST /admin/local/accounts/{accountID}/unblock` | `unblockLocalAccount` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/local/reports/reservations` | `getAdminLocalReservationsReport` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/local/reports/finance` | `getAdminLocalFinanceReport` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/local/reports/disputes` | `getAdminLocalDisputesReport` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/local/reservations` | `listAdminLocalReservations` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/local/reservations/{reservationID}` | `getAdminLocalReservation` | Bearer; permisos en Go | FE-11 — Administración y colas | Implementado; aceptado y fusionado en PR #286; la cobertura viva individual se detalla en FE-11 |
| `GET /admin/local/damage-claims` | `listOpenLocalSyntheticDamageClaims` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 aprobado y fusionado |
| `GET /admin/local/damage-claims/{claimID}` | `getAdminLocalSyntheticDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 aprobado y fusionado |
| `GET /admin/local/damage-claims/{claimID}/evidence/{evidenceID}` | `getAdminLocalDamageClaimEvidence` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 aprobado y fusionado |
| `POST /admin/local/damage-claims/{claimID}/resolution` | `resolveLocalSyntheticDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 aprobado y fusionado |

## Trazabilidad FE-03 — [PR #276](https://github.com/HernanEspinozaDev/espaciGo/pull/276) aprobado y fusionado

| Operación | Pantalla / componente | Validación | Evidencia visual |
| --- | --- | --- | --- |
| `GET /local/booking-trial/catalog` | `/explorar`: búsqueda autenticada, categorías y perfiles, filtros tipados, paginación opaca, plano sintético | 41 pruebas Angular; editar criterios invalida la petición/cursor; 401 solo expira el Bearer origen; respuesta A descartada tras logout/login B; `int64` CLP preservado como decimal exacto | [Explorar escritorio](evidencia/FE03_explorar_escritorio.png) · [móvil](evidencia/FE03_explorar_movil.png) |
| `GET /local/booking-trial/catalog/{spaceId}` | `/espacios/:id`: detalle seguro sin dirección, propietario ni galería privada | Recorrido de componente con sesión real y cuenta sintética; respuestas tardías condicionadas por espacio y sesión | [Detalle escritorio](evidencia/FE03_detalle_escritorio.png) · [móvil](evidencia/FE03_detalle_movil.png) |
| `GET /local/booking-trial/catalog/{spaceId}/availability-options` | Selector de fecha local, duración e intervalos UTC distintos para horas DST repetidas | `America/New_York` de prueba: las dos 01:30 muestran offsets diferentes; duración válida al cambiar hora/día/mes; selector vacío anula selección y cotización | [Detalle escritorio](evidencia/FE03_detalle_escritorio.png) · [móvil](evidencia/FE03_detalle_movil.png) |
| `POST /local/booking-trial/quotes` | Cotización real de Go con snapshot CLP exacto y aviso de ensayo | `int64` > `Number.MAX_SAFE_INTEGER`; selección devuelta comparada con espacio e intervalo actual; cotización no crea reserva | [Detalle escritorio](evidencia/FE03_detalle_escritorio.png) |

La UI reutiliza `M04Local` y sus tipos generados para categorías/perfiles. Catálogo, disponibilidad y cotización se leen como texto: los literales de CLP se convierten en cadenas antes de parsear el JSON y se formatean con `BigInt`; filtros CLP viajan como cadenas de consulta. No se aplica `Number` a importes. El Bearer se adjunta en memoria por solicitud, no se registra y no pasa por URL. Go sigue siendo autoridad para roles, permisos, disponibilidad, zona IANA, horarios y snapshot de cotización. El mapa solo representa `x/y` normalizados y está rotulado como sintético.

**Validación:** 41 pruebas Angular, `api:check`, build/prerender y prueba M05/M06 Go contra PostgreSQL temporal en `tmpfs`; cuenta y datos sintéticos. Incluye errores 401/403/404/409/422, permiso de arrendatario, DST, cotización y aislamiento de sesión. Sin pruebas destructivas en el volumen persistente, sin proveedores reales y con el mock conservado. FE-03 fue aceptado y fusionado; FE-04 comienza a continuación.

## Trazabilidad FE-04 — cuenta y privacidad (aceptado; PR #278 fusionado)

| Operación Go | Pantalla / comportamiento | Validación | Evidencia visual |
| --- | --- | --- | --- |
| `GET/PUT /profile` | `/mi-cuenta`: ver y actualizar nombre/teléfono del titular; el 404 de perfil muestra estado vacío y habilita creación por PUT | SDK OpenAPI generado y prueba de componente/sesión: cuenta nueva → 404 → creación → estado confirmado; solicitudes y exportaciones siguen disponibles | [Escritorio](evidencia/FE04_cuenta_escritorio.png) · [móvil](evidencia/FE04_cuenta_movil.png) |
| `GET/PUT/DELETE /profile/photo`, `GET /profile/photo/content` | Foto sintética privada: metadatos, contenido autenticado, generar y retirar | Sin campos de subida; contenido mediante Blob autenticado; URLs revocadas al cerrar sesión, cambiar recurso y destruir vista; vacío/retirado no reconstruye datos | [Escritorio](evidencia/FE04_cuenta_escritorio.png) · [móvil](evidencia/FE04_cuenta_movil.png) |
| `GET/PUT/DELETE /payout-account` | Referencia fake de cuenta de cobro, sujeta a elegibilidad Go | No acepta datos bancarios; idempotencia y Bearer comprobados; Go valida elegibilidad | [Escritorio](evidencia/FE04_cuenta_escritorio.png) |
| `GET/POST /rights-requests` | Solicitudes de acceso y supresión con estado devuelto por Go | Titular autorizado y aislamiento; supresión queda como solicitud revisable, no borrado inmediato | [Escritorio](evidencia/FE04_cuenta_escritorio.png) · [móvil](evidencia/FE04_cuenta_movil.png) |
| `GET /privacy/export`, `GET /privacy/export/archive` | Descargar JSON propio y archivo ZIP completo | Angular: JSON/ZIP Blob y cabecera PK; Go con PostgreSQL temporal: contenidos, scope del titular, foto/ref fake presentes y credenciales/otra cuenta excluidas | [Escritorio](evidencia/FE04_cuenta_escritorio.png) |

**Validación:** 55 pruebas Angular en 13 archivos; `api:check`; build/prerender. Pruebas de componente con IdentitySession real cubren el perfil 404 y alta, guardado concurrente con solicitud/foto, GET de perfil obsoleto que termina después del PUT confirmado y la publicación/descarto del aviso de foto según recarga, sesión y destrucción de vista. El GET obsoleto no cancela la recarga ni bloquea acciones; solo no aplica perfil/nombre/teléfono viejos. Pruebas Go enfocadas de foto sintética/cuenta fake, autorización por titular, aislamiento de dos cuentas y exportaciones con PostgreSQL temporal y Mailpit desechable. Se mantienen casos de Bearer vigente 401 (limpia estado), 401 tardío de A después de B (no expira B), logout/login, respuestas tardías, limpieza de Blob URLs, descarga/estructura JSON y ZIP. Capturas marcadas con respuestas/API sintéticas.

**Límite de automatización:** no se automatizó el selector/diálogo del sistema de archivos del navegador para comprobar escritura física de la descarga. Angular inspecciona el Blob de descarga (incluida firma ZIP) y la prueba Go abre/verifica el ZIP y sus contenidos; el clic de descarga se comprueba en componente. El mock se conserva y Go no se modifica.

## Trazabilidad FE-05 — KYC sintético ([PR #279](https://github.com/HernanEspinozaDev/espaciGo/pull/279), commit `3405f05`; aprobado y fusionado)

Ruta privada `/mi-cuenta/verificaciones`, guard de sesión, render CSR y metadato `noindex,nofollow`. La vista separa acceso HTTP (`VerificationApi`), estado del recorrido (`VerificationPage`) y plantilla. Bearer sólo en memoria; las sesiones y selecciones tienen generación propia para descartar respuestas tardías. Una respuesta 401 sólo expira el bearer vigente. Los fixtures de evidencia se solicitan a Go con el literal permitido; el PNG se obtiene por petición autenticada y se representa con Blob URL temporal, revocada al seleccionar otro caso, cambiar/cerrar sesión o destruir la vista. `file_available=false` representa archivo retirado y no reconstruye contenido.

| Recorrido | Comportamiento visible | Pruebas/evidencia |
| --- | --- | --- |
| Casos propios KYC/KYB y detalle | Estados `en_revision`, `aprobada`, `rechazada`, `revocada`, fechas, proveedor y motivo vienen de Go | `VerificationPage` + `IdentitySession` real; lista/detalle/historial y aislamiento |
| Crear / corregir | Crear caso sintético; el rechazo permite reenviar código compatible y Go crea caso nuevo | Claves idempotentes y códigos limitados al esquema; sin privilegios concedidos por UI |
| Elegibilidad | Muestra KYC/KYB según respuesta efectiva, independiente de preferencia y roles | Elegibilidad no inferida en navegador |
| Evidencia | Metadatos de fixture y vista privada autorizada; vacío/retirado sin reconstruir | No hay file input ni subida de bytes; tercero recibe rechazo/ocultación Go |
| Sesiones y concurrencia | Cambio A→B descarta listados/detalle/PNG de A; 401 actual limpia, 401 tardío no expira B | Suite Angular focused/full; Go handler owner-scoped |

**Validación:** 7 pruebas KYC enfocadas; suite Angular consolidada: 62 pruebas en 14 archivos; `api:check`; build/prerender; pruebas Go del handler y del repositorio de verificaciones con PostgreSQL temporal en `tmpfs`. Cubren titular/tercero, estados, retry, evidencia, 401 actual/tardío, cambio de cuenta, respuestas de PNG fuera de orden y Blob URL al cambiar caso/logout/destruir vista. Las capturas reflejan UI Angular con fixtures de API sintéticos locales: [escritorio](evidencia/FE05_kyc_escritorio.png) · [móvil](evidencia/FE05_kyc_movil.png). No se probaron documentos reales ni se usó volumen persistente. FE-04 y FE-05 aceptados/fusionados en PR #278 y #279. FE-06 se registra a continuación.

## Trazabilidad FE-06 — Oferta y calendario ([PR #280](https://github.com/HernanEspinozaDev/espaciGo/pull/280); aceptado y fusionado)

El panel `/panel/espacios` organiza datos, tarifas, calendario y galería con controles responsive Terracota + Crema. La lista, detalle, guardados, tarifario, simulaciones, disponibilidad, bloques, zona y horario semanal consultan Go con Bearer en memoria. La UI conserva estados vacíos, reconoce 401/403/404/409/422, pide confirmación en retiro de imagen y eliminación, y distingue publicar/ocultar/eliminar. Tarifas y respuestas con importes usan el parser/serializador exacto; fechas de pared se convierten desde zona IANA y DST ambiguo requiere seleccionar ocurrencia. Las lecturas comparan sesión/generación/recurso; escrituras confirmadas invalidan lecturas anteriores. La galería solo genera el fixture permitido, usa claves idempotentes y descarga contenido con Bearer en Blob URL temporal.

| Recorrido | Comportamiento y pruebas | Estado |
| --- | --- | --- |
| Propietario / tercero | `IdentitySession` real; listado propio y detalle no propietario oculto por 404 de Go | Componentes Angular; integración Go con PostGIS efímero cubre propietario, catálogo/detalle/cotización tercero y denegación de escrituras ajenas |
| Borrador y publicación | Perfil versionado, PUT completo fusionado con el detalle más reciente; PUT de contenido publicado solo contiene campos cambiados; publicación, ocultamiento y baja separados | Cubierto por pruebas de componente/API |
| Tarifas y modalidad | JSON exacto sin `Number`/redondeo; versión y simulación, cambio de unidad invalida simulación e intervalos | Parser exacto y unidad cubiertos; Go conserva validación |
| Calendario | zona IANA, DST inválido/ambiguo, disponibilidad, bloqueos y semana; 409 se representa como conflicto de Go | DST y conflicto cubiertos; rutas de API generan cliente/compilan |
| Galería | fixture sintético permitido, Bearer en contenido, idempotencia y revocación del Blob URL; respuesta tardía de A no llega a sesión B | Pruebas API y de componente |

**Validación de revisión:** 22 pruebas FE-06 enfocadas; Angular consolidado 95 pruebas / 17 archivos; `npm run api:check`; `npm run build` y prerender de 3 rutas. Se añadieron regresiones del 401 de configuración paralelo al cambiar de recurso y del 401 de Bearer antiguo tras logout/login, conservando sesión nueva. Go: `scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/spaces` pasó con PostgreSQL/PostGIS efímero (`tmpfs`). El proyecto/volumen Docker exclusivo `espacigo_fe06` verificó vía Go 10 anfitriones con rol explícito/KYC sintético, publicaciones, tarifa y galería/portada, además de borrador, simulación y calendario. Como el registro no concede roles, el rol anfitrión se asignó explícitamente como fixture solo en la base desechable; KYC se revisó por el flujo administrativo local Go; 3 arrendatarias verificaron catálogo, detalle, cotización, denegación de detalle privado y solicitud con respuestas HTTP aceptadas o 409 de conflicto confirmadas por Go. Las cuentas sintéticas y su receta están en `.local/testing/cuentas.md`, con permisos 0600 y exclusión local; no se incluyeron secretos en evidencias. Capturas del panel Angular conectado a Go: [escritorio](evidencia/FE06_oferta_escritorio.jpg) y [móvil](evidencia/FE06_oferta_movil.jpg). La vista móvil se comprobó a 390×844 CSS px; la superficie de captura del navegador integrado limita la imagen a 375×260 px. El stack solo usó `espacigo_fe06_pgdata`; el volumen persistente no se tocó, Go conserva sus rutas y el mock sigue disponible.

## Trazabilidad FE-02 — [PR #275](https://github.com/HernanEspinozaDev/espaciGo/pull/275) aprobado y fusionado

| Operación | Pantalla / componente | Validación | Evidencia visual |
| --- | --- | --- | --- |
| `GET /auth/terms`, `POST /auth/register` | Registro: términos versionados, preferencia inicial, validación accesible | Script M01 con términos vigentes, aceptación, unicidad y error de duplicado; UI conectada a Go | [Registro móvil](evidencia/FE02_sesion_movil.jpg) |
| `POST /auth/verification`, `POST /auth/verification/reissue` | Verificar correo: consumición de token y reemisión | Mailpit + API: verificación/reemisión, token inválido/reutilizado | [Registro móvil](evidencia/FE02_sesion_movil.jpg) |
| `POST /auth/login`, `GET /auth/session` | Acceso y mi cuenta; rol/modo restringido confirmados por Go | Cuenta sintética, login, consulta sesión; test de rutas por rol y modo restringido | [Sesión escritorio](evidencia/FE02_sesion_escritorio.jpg) |
| `POST /auth/logout` | Navegación global / cerrar sesión | Revocación API, limpieza de estado privado y respuesta tardía aislada en tests | [Sesión escritorio](evidencia/FE02_sesion_escritorio.jpg) |
| `POST /auth/password/recovery`, `POST /auth/password/recovery/consume`, `POST /auth/password/change` | Recuperar acceso y seguridad de cuenta | Script M01 + Mailpit: respuesta anti-enumeración, token inválido/reutilizado, cambio, revocación y nuevo login | API + Mailpit; sin captura que exponga tokens |

Cliente generado reutilizado desde `frontend/src/generated/api`. El Bearer vive en una señal en memoria; no se guarda en storage ni se agrega a rutas. La capa de acceso añade explícitamente Authorization a operaciones protegidas debido al orden de construcción de request del cliente generado, cubierto por prueba de `HttpTestingController`. Roles no se infieren de la preferencia de uso. Identidad y cuenta se renderizan en CSR/noindex. Las páginas futuras solo muestran avisos; sus funciones no se declaran implementadas.

Ejecución: 26 pruebas Angular enfocadas; build/prerender; generación OpenAPI y compilación TS; script `scripts/verify-m01.py` en proyecto Compose desechable con PostgreSQL y Mailpit; escaneo de logs del backend sin secretos. Las capturas representan UI conectada a servicios locales y cuentan sintéticas.

## Trazabilidad FE-07 — Reserva y pago ([PR #282](https://github.com/HernanEspinozaDev/espaciGo/pull/282); aceptado y fusionado)

| Operación | Pantalla / componente | Validación | Evidencia |
| --- | --- | --- | --- |
| Listar y solicitar reserva | Bandeja por participante y detalle desde cotización | Solicitud idempotente, duplicado misma clave, titular/tercero | Bandeja escritorio/móvil |
| Consultar detalle e historial | Detalle privado con transiciones Go | Aislamiento por participante y sesión | Detalle escritorio |
| Pago fake | Resultado éxito/rechazo/timeout | Exactitud int64, 504 incierto y reintento misma clave/payload | Pruebas FE-07 y recorrido Go |
| Decidir reserva | Acciones de anfitrión aprobar/rechazar | Host aprueba/rechaza; roles y estado confirmados por Go | Decisión anfitrión móvil |
| Política, cancelación y devolución | Vista de cancelación/devolución simulada | Elegibilidad y obligación confirmadas por Go; API fake éxito/fallo/timeout | Detalle y pruebas/API Go |

Páginas privadas CSR/noindex. Bearer y claves idempotentes solo en memoria; el parser procesa respuesta textual para conservar int64 exacto. Todo intento no asentado conserva clave/payload en memoria por cuenta/reserva/operación, incluso al reautenticar la misma cuenta; no contiene Bearer. Avisos inciertos sobreviven a refresh fallido o aún pendiente y se limpian cuando Go confirma el estado final. Los motivos y controles privados se limpian al cambiar sesión/reserva, y respuestas tardías se descartan por generación/sesión. Go confirma acceso y transición; la UI no concede roles.

Validación: 12 pruebas de componente FE-07 con IdentitySession real y una prueba de store; suite Angular consolidada de 112 pruebas en 19 archivos; npm run api:check; npm run build con prerender de 3 rutas. Las regresiones cubren 4xx de un Bearer previo tras logout/login de la misma cuenta, devolución/cancelación con POST pendiente y GET posterior completada, lectura antigua de devolución fallida posterior a una devolución exitosa, respuestas de cancelación fuera de orden, recarga fallida o sin campos de devolución y protección monotónica del snapshot confirmado. El stack aislado espacigo_fe07 con PostgreSQL y Mailpit confirmó cinco cuentas sintéticas, solicitud idempotente, rechazo de tercero, pago fake exitoso/rechazado/timeout, decisión de anfitrión, cancelación y devolución fake según Go. La UI Angular conectada completó login, búsqueda/cotización, solicitud, pago, aprobación, cancelación y devolución pendiente → completada confirmadas por Go. Go devuelve operation_id/importe pendientes desde la respuesta de cancelación pero no los repite en el GET detalle; Angular conserva la respuesta autoritativa en un snapshot temporal en memoria y abre la devolución con esos datos. La respuesta del POST se aplica antes de recargar; un GET posterior reemplaza el estado y snapshot si Go devuelve campos confirmados, mientras que GET sin esos campos reutiliza la confirmación guardada. Una recarga fallida mantiene la confirmación del POST. El estado visible y el snapshot quedan protegidos frente a respuestas fuera de orden. No se usaron proveedores ni dinero real.

Credenciales sintéticas y receta: .local/testing/fe07-cuentas.md, modo 0600 y excluidas en .git/info/exclude; no versionar ni publicar. El stack FE-07 no tocó volúmenes persistentes. Evidencia: [devolución completada escritorio](evidencia/FE07_devolucion_completada_escritorio.jpg), [devolución completada móvil](evidencia/FE07_devolucion_completada_movil.jpg), [bandeja escritorio](evidencia/FE07_reservas_escritorio.jpg), [bandeja móvil](evidencia/FE07_reservas_movil.jpg), [detalle escritorio](evidencia/FE07_reserva_detalle_escritorio.jpg), [decisión móvil](evidencia/FE07_decision_anfitrion_movil.jpg). UI comprobada a 1440×1000 y 390×844 CSS px; la captura móvil del navegador integrado es limitada. FE-06 aceptado/fusionado en PR #280; FE-07 aceptado/fusionado en PR #282; FE-08 aceptado y fusionado en PR #283; FE-09 aceptado y fusionado en PR #284 (`0d3bac8`); FE-10 iniciado desde `main` actualizado.

## Trazabilidad FE-08 — Contrato y operación (PR #283 aprobado y fusionado)

| Operación Go | Interfaz Angular | Validación enfocada | Evidencia |
| --- | --- | --- | --- |
| Crear/consultar, firmar/rechazar contrato y descargar PDF privado | Panel del detalle de reserva | Participantes confirmados, tercero denegado, transición de firma y PDF autenticado | Stack Go desechable; prueba de API/componente |
| Listar operaciones, check-in, check-out, recepción, descargar evidencia privada | Panel de llegada/salida/recepción | Arrendatario/anfitrión según operación, tercero 403/404, estados confirmados, PNG sintético privado | Stack Go desechable; prueba de API/componente |

**Estado FE-08:** aprobado y fusionado en [PR #283](https://github.com/HernanEspinozaDev/espaciGo/pull/283), merge `503d19359e94c9f187bb955047c150aef8b97507`; HEAD `e85e8cc493589badb34dd305dcf128c57982667c`. Implementación base `0bf20393e2c198479abbdfa3784d8706082b9240`; correcciones `6aceba2d36a99f14cb7c31a52f3e5d7cd1a4e514`, `608a6bfcfe4b56f9696084b245c83767aaf1dc88` y `e85e8cc493589badb34dd305dcf128c57982667c`. Autor HernanMEC; revisor HernanEspinozaDev. Componentes: `BookingLifecycleApi` y `BookingLifecyclePanel`, integrados al detalle existente. El acceso se valida con IDs de participante y estados devueltos por Go; la API sigue imponiendo autorización. Bearer solo en memoria, intentos idempotentes en memoria sin Bearer; lecturas obsoletas se descartan por sesión/recurso/generación; URL de Blob PDF/PNG se revoca al cerrar sesión, cambiar recurso o destruir la vista.

**Validación:** stack aislado `espacigo_fe08` con PostgreSQL y Mailpit, usando cuentas sintéticas autorizadas. En Angular conectado a Go: anfitrión y arrendatario firmaron, se solicitó el PDF privado, se registró check-in/out con comentarios sintéticos no vacíos y el anfitrión confirmó recepción; el padre recargó el detalle y reflejó los estados posteriores devueltos por Go. La UI también rechazó la versión. Go confirmó tercero denegado para reserva, contrato, PDF, operaciones y evidencia; ambos participantes pudieron consultar el PDF/evidencia privados. Pruebas integradas con `BookingDetailPage`, `BookingLifecyclePanel` e `IdentitySession` real cubren el recorrido, rechazo, retry incierto con payload/clave originales, 4xx tardío frente a intento nuevo y aislamiento de formulario/efectos tardíos.

**Evidencia visual:** [escritorio](evidencia/FE08_flujo_angular_go_escritorio.png) y [compacta](evidencia/FE08_flujo_angular_go_movil.png), páginas completas del detalle con estado final e historial Go. La captura fue realizada con el navegador Firefox local conectado a `localhost`; viewport escritorio 1366×682 CSS px y compacto 500×758 CSS px (488 px de ancho de documento, sin overflow horizontal). Firefox Snap limita el ancho mínimo a 500 px, así que 390 px no quedó comprobado. La prueba visual obtiene sesión/Bearer desde Go y la coloca solo en memoria de Angular para aislar el login; el login UI se cubre por las pruebas de identidad existentes. Go confirmó las respuestas PDF/evidencia privadas y el botón Angular de PDF se activó, pero el navegador no permite afirmar que el sistema operativo guardó el archivo. No se usaron proveedores externos ni volúmenes persistentes.

**Regresión consolidada:** 125 pruebas Angular / 22 archivos; `npm run api:check`; `npm run build` y prerender de 3 rutas. Se corrigió el encabezado de request que impedía la preflight CORS hacia Go.

Cuentas sintéticas y receta de recreación: `.local/testing/fe08-cuentas.md` y `.local/testing/provision_fe08.py` (modo 0600, `.local/testing/` excluido por Git y `.git/info/exclude`; no versionar ni publicar). Stack nuevo en puerto 18430/18431/18432 y volumen aislado `espacigo_fe08_pgdata`; no se alteró ni eliminó volumen persistente preexistente. La regeneración usa `scripts/local-docker-stack.sh` con prefijo/proyecto/volumen `espacigo_fe08`; después ejecuta el provisionador local; credenciales no se incluyen en logs o GitHub. El mock queda intacto.

## Escenarios que deben trasladarse del mock

| Protección existente | Destino Angular / aceptación |
| --- | --- |
| Sesión y cuenta/generación | FE-02/04/11: logout y nuevo login con misma cuenta, 401/403 tardíos, exportaciones. |
| Selección/revisión de espacio | FE-03/06: categorías/perfiles fuera de orden, galería/portada, cursor/filtros, cotización ajena a selección. |
| Formularios parciales | FE-06: enviar solo campos cambiados y conservar tarifa concurrente; montos sin redondeo. |
| Reserva/operaciones | FE-07/08: resultado tardío no modifica otra reserva; idempotencia y estados terminales. |
| Conversación/lectura | FE-09: contadores no cambian selección, propios no cuentan, cursor monotónico y respuesta vieja descartada. |
| Reclamos/evidencia | FE-10: recuperación de PNG sin reenviar descargo, aislamiento y listas vacías tras retirada. |
| Admin/auditoría | FE-11: cambiar filtros limpia cursores/detalle; 403 limpia y refresca roles, 401 vigente limpia sesión. |
| Descargas y dinero | FE-04/08/10/11: archivos auténticos autorizados, null tras purga, estados fake confirmados y enteros exactos. |

## Condición para eliminar el mock

Todas las operaciones de interfaz deben estar implementadas y verificadas en Angular o tener una exclusión técnica explícita y aceptada. Ninguna función puede figurar como completada por existir solo en el mock. FE-14 registra paridad; FE-15 adapta runners y elimina el mock después de la aceptación del usuario.


## Trazabilidad FE-09 — Conversación, lectura, reseñas y reportes (PR #284 aprobado y fusionado)

| Operación | Interfaz | Evidencia de comportamiento |
| --- | --- | --- |
| Páginas, envío y lectura de mensajes | Conversación dentro de reserva | Stack Go y Angular: dos participantes, 40 mensajes; UI cargó la página anterior desde el cursor; tercero denegado; texto terminal legible y envío terminal HTTP 409 |
| Contador no leído | Bandeja y panel | Go incrementó y limpió contador por cursor; las pruebas evitan aplicarlo tras cambio de sesión/reserva |
| Reseña de reserva y reporte | Detalle de reserva | Ambos participantes pudieron reseñar; host reportó reseña y Go la marcó reportada |
| Reseñas públicas y reputación propia | Detalle/cuenta | Se activó publicación propia con Go; `GET /spaces/{spaceID}/reviews` HTTP 200 con una reseña; reputación consultada por el titular |

Pruebas Angular: 141 en 24 archivos; regresiones de sesión/401 tardío, respuestas fuera de orden, paginación concurrente con refresh y éxito/error obsoleto, terminal solo lectura, texto como texto, lectura tras render, clic DOM de reintento idempotente con formulario limpio y resumen público actualizado después de reseña con descarte de lectura anterior. `api:check` y `build` con prerender de 3 rutas pasan. Revisión autenticada en navegador integrado: Angular HTTP 200, Go readiness HTTP 200; sin `ERR_BLOCKED_BY_CLIENT`. Arrendatario: 40 mensajes, paginación UI operativa, intento incierto conserva el formulario original y su reintento produjo una sola fila Go (#40). Anfitrión: reseña reportada, otra publicada y vista terminal solo lectura; resumen público 5/5, 1 reseña. Evidencia capturada desde Angular conectado a Go: [conversación desktop 1413×1004](evidencia/FE09_conversacion_escritorio.jpg) · [móvil 473×824](evidencia/FE09_conversacion_movil.jpg) · [reintento desktop 1413×1004](evidencia/FE09_reintento_escritorio.jpg) · [reseñas/reportes desktop 1413×989](evidencia/FE09_reportes_escritorio.jpg) · [móvil 473×824](evidencia/FE09_reportes_movil.jpg). Los 404 iniciales correspondían a un ID obsoleto que no pertenecía a esa cuenta; se corrigió consultando reservas actuales de Go. No se compartieron credenciales.

FE-09, FE-10 y FE-11 aprobados y fusionados en PR #284, #285 y #286.

**Aceptación:** FE-09, FE-10 y FE-11 aprobados y fusionados en PR #284, #285 y #286.

## Trazabilidad FE-10 — Disputas y finanzas ([PR #285 aprobado y fusionado](https://github.com/HernanEspinozaDev/espaciGo/pull/285))

| Flujo/operación Go | Interfaz Angular | Prueba/evidencia | Estado/límite |
| --- | --- | --- | --- |
| Incidencias e historial; reclamo, descargo y evidencia sintética | Detalle de reserva → Disputas y finanzas | Go desechable: anfitrión abre incidencia/reclamo; arrendatario envía descargo; tercero obtiene 404; administrador cierra incidencia y reclamo | Implementado; evidencia solo privada PNG sintética |
| Evidencia privada de participantes y administración | Controles de descarga con Bearer y Blob temporal | La descarga autenticada se verificó; logout dejó cero imágenes privadas en DOM y revocó el Blob URL | Implementado; sin archivos reales/arbitrarios |
| Garantía, deducción y conciliación | Panel de reserva y cola administrativa | Go desechable confirmó decisión de CLP 1.000; integración con reloj sustituible completó autorización, captura, liberación, conciliación y replay | Implementado; no hay movimiento monetario |
| Liquidación y comprobante | Panel de reserva y conciliación administrativa | Test Go con reloj sustituible cruzó check-out + 24 h, confirmó liquidación, reconciliación, transición terminal y comprobante único | Éxito probado en base PostgreSQL/PostGIS temporal; el fixture del navegador conserva sus fechas/estado originales |

FE-09 aceptado y fusionado en PR #284, merge `0d3bac8a06457899239a49274b54ad6807846f42`. Angular 4200 HTTP 200 y Go 18280 `/health/ready` HTTP 200. Stack desechable `espacigo_fe10_20261012` con volumen nuevo; anfitrión/arrendatario participantes y admin aceptados por Go; tercero denegado 404 para disputas/reclamos/garantía. La incidencia se abrió y cerró; reclamo y descargo con evidencia PNG sintética se consultaron con Bearer; admin resolvió y Go confirmó decisión acogida con deducción de 1.000 CLP, evidencia y estado `pendiente`; sin dinero real ni captura aplicada. Comprobante GET 404; el POST settlement recibió HTTP 404 para esta reserva. El caso sigue en `en_disputa` con decisión financiera pendiente y además aún no cumple las 24 h desde check-out, así que no se intenta forzar una liquidación. No se forzó tiempo y no se tocó volumen persistente.

Evidencia Angular conectada a Go: [participante escritorio 1425×2787](evidencia/FE10_disputas_escritorio.jpg), [participante móvil 375×4050](evidencia/FE10_disputas_movil.jpg), [cola admin escritorio 1425×2289](evidencia/FE10_admin_escritorio.jpg), [cola admin móvil 375×2703](evidencia/FE10_admin_movil.jpg), [panel actual desktop 1265×712](evidencia/FE10_admin_actual_escritorio.jpg) y [móvil 375×812](evidencia/FE10_admin_actual_movil.jpg). Las capturas de panel actual son viewports; las primeras son páginas completas.

Correcciones de revisión: commit `b2da3243becb9db0fa10828baf5cae9782af474f`. Regresión Angular: 149 pruebas en 26 archivos, incluidas selección admin A→B con GET tardío, sesión admin A→B con intento incierto, aislamiento por cuenta y respuestas tardías exitosas/4xx sin borrar la instancia nueva. Test Go: `go test ./internal/adapters/postgres/booking -count=1` en PostgreSQL/PostGIS descartable, con reloj inyectado; tres pruebas de integración pasan y verifican garantía, liquidación/reconciliación y comprobante exitosos. `api:check` pasó con `ng serve` detenido; build/prerender pasó con 3 rutas. FE-10 y FE-11 aceptados y fusionados en PR #285 y #286; FE-12 comienza ahora.


## Trazabilidad FE-11 — Administración completa ([PR #286 aprobado y fusionado](https://github.com/HernanEspinozaDev/espaciGo/pull/286))

| Área | Operaciones Go | Estado y evidencia |
| --- | --- | --- |
| Cola KYC y evidencia privada | listar pendientes/rechazadas, revisar/revocar, listar/leer/eliminar evidencia | Angular conectado: lectura autenticada del PNG sintético mediante Bearer/Blob URL y aprobación confirmada por Go. La consulta posterior de la cola no devuelve el caso ya aprobado. |
| Privacidad y reportes de reseñas | listar/revisar/ejecutar supresión, purga de retención, moderar reporte | Angular conectado: Go confirmó revisión y ejecución de supresión sintética; el recurso deja de estar disponible. Reseña sintética reportada y moderada desde la UI; Go devolvió cola vacía. |
| Avisos/reapertura | avisos locales y credenciales, reapertura | Panel y formularios de motivo implementados; regresiones de payload incierto conservan clave, payload y `correlation_id`. No había aviso terminal disponible para probar reapertura viva. |
| Cuentas, publicaciones y reservas | consulta de cuenta/roles, bloquear/desbloquear, publicaciones y reservas | Angular conectado: cuenta sintética bloqueada y desbloqueada desde la UI; Go confirmó estado activo al consultar después. Roles permanecen Go-confirmados. |
| Auditoría y exportación | eventos, cursor, exportación | Exportación JSON desde Angular descargada y parseada: 14 eventos. El intervalo se conserva en la paginación/exportación por regresión; el cursor no pudo probarse en vivo porque no se alcanzaron 25 eventos (tamaño de página 25). |
| Reportes agregados | reservas, finanzas y disputas | Vistas Angular implementadas con filtros de fechas/zona. No se volvió a recorrer en vivo esta sección durante esta corrección. Capturas del panel autenticado: [escritorio 1440×914](evidencia/FE11_admin_escritorio.jpg) y [móvil 500×758](evidencia/FE11_admin_movil.jpg), `scrollWidth <= innerWidth`; Firefox headless limita el ancho mínimo a 500 px. |

Regresión Angular: `npm test -- --watch=false` — 163 pruebas / 28 archivos; incluye 401/403 vigente/tardío, cambio de sesión, intentos idempotentes, paginación, `correlation_id` original y 403 vigente con revocación/limpieza y refresh de capacidades. `GET /health/ready` de Go y Angular respondieron HTTP 200 en el stack FE-11 desechable; se registró la ruta Go de moderación existente que no estaba montada y pasa `go test ./cmd/api -run TestReputationRoutesDoNotInterceptBookingOperations`. La prueba visual muestra cola KYC con evidencia privada en desktop y auditoría en mobile; la captura mobile tiene 500 px de ancho por límite del navegador headless. `npm run api:check` pasó con `ng serve` detenido y `npm run build` completó prerender de 3 rutas.

FE-10 y FE-11 aceptados y fusionados en PR #285 y #286. FE-12 y FE-13 aceptados y fusionados en PR #287 y #288. FE-14 aceptado explícitamente sobre `main` `d02f9c8` después de revisar la evidencia y fusionar #290/#291; FE-15 en curso.


## Trazabilidad FE-12 — Consolidación visual, responsive y accesibilidad

| Área | Cambio / comprobación | Evidencia y límite |
| --- | --- | --- |
| Navegación y foco | El Router real mueve el foco al `h1` después de navegación final y al reutilizar `/reservas/:id`. Una página exacta usa `aria-current="page"`; Administración/Mi cuenta actúan como ubicación de sección en rutas hijas. Menú móvil: Escape lo cierra y devuelve foco al disparador. | 2 regresiones de Router real comprueban cambio de parámetro, una sola página actual en disputas y ubicación de sección en cuenta/admin. axe-core en shell. |
| Referencia A · portada | Portada navegable autenticada en el stack FE-11 sintético; visualización móvil/escritorio, texto largo e ilustración adaptable. | [Móvil 390×844](evidencia/FE12_A_portada_movil.jpg) · [Escritorio 1365×900](evidencia/FE12_A_portada_escritorio.jpg). La superficie de captura entrega JPEG 375×812 y 1350×890, respectivamente. |
| Referencia B · detalle y cotización | Ficha sintética real de oficina, CLP 12.500/h, zona `America/Santiago`, selector y estado sin horarios vigentes para la fecha consultada. La UI deja Cotizar deshabilitado. | [Detalle móvil 390×844](evidencia/FE12_B_detalle_movil.jpg) · [Detalle/cotización escritorio 1365×900](evidencia/FE12_B_detalle_escritorio.jpg), JPEG 375×812 y 1350×890. El stack confirmó catálogo y detalle; no había intervalo disponible para una cotización exitosa en esta revisión. |
| Referencia C · reservas privadas | Sesión de anfitrión confirmada por Go; 8 solicitudes largas en estados final, rechazado, cancelado y vencido. Cuenta privada adicional revisada con estado de perfil/foto/cobro vacío. | [Reservas móvil 390×844](evidencia/FE12_C_reservas_movil.jpg) · [Reservas escritorio 1365×900](evidencia/FE12_C_reservas_escritorio.jpg) · [Cuenta móvil](evidencia/FE12_C_cuenta_movil.jpg) · [Cuenta escritorio](evidencia/FE12_C_cuenta_escritorio.jpg). JPEG de viewport: 375×812 y 1350×890. `innerWidth=390`, `clientWidth=scrollWidth=375` (scrollbar de 15 px); desktop sin overflow. |
| Teclado, foco y contraste | Tab/Escape en el menú mobile mantiene el foco visible; el indicador azul de 3 px mide 6.28:1 contra Crema. Muestreo de hojas visibles de texto en `/mi-cuenta`: 0 elementos bajo 4.5:1 (texto normal) o 3:1 (texto grande). | AXE no reportó infracciones A/AA en el shell. Esta comprobación acotada no equivale a certificación WCAG ni a revisión con lector de pantalla/zoom 400%. |
| Carga, vacío, error y desconexión | Go readiness visible como disponible; ficha sin horario, perfil/foto/cuenta de cobro vacíos y estados terminales privados de reservas. Aviso offline y reintento de readiness cubiertos en prueba de componente. | En esta pasada visual no se interrumpió Go para generar un error de red; los estados error/offline se verificaron mediante las regresiones existentes, no se capturaron como pantallas. |

**Regresión final de FE-12:** Angular — 168 pruebas en 28 archivos; `npm run api:check` pasó con `ng serve` detenido; `npm run build` compiló y prerenderizó 3 rutas. FE-12 aprobado y fusionado en #287; FE-13 se aceptó y fusionó en #288.

### Revisión visual complementaria del tercer hilo de #287

Stack conectado: Angular `127.0.0.1:4200` HTTP 200 y Go `127.0.0.1:18380/health/ready` HTTP 200. Se usaron sesiones sintéticas: anfitrión (`arrendador` confirmado por Go) y administrador (`administrador` confirmado por Go). No se repitieron mutaciones de negocio; los estados se leyeron desde Go y las capturas no contienen correos ni contraseñas.

| Ruta / rol | Estado y contenido observado | Teclado, foco, contraste y responsive | Evidencia |
| --- | --- | --- | --- |
| `/panel/espacios` · anfitrión | Un borrador propio de oficina (36 m²), con descripción extensa y campos de tarifa/atributos visibles. | Tab enfoca «Ver catálogo» con anillo azul de 3 px. Contraste: 0 fallos en 48 textos móvil y 54 escritorio; el texto claro sobre el degradado Terracota conserva al menos 5,65:1 frente al extremo más claro. Móvil `innerWidth=390`, `clientWidth=scrollWidth=375`; escritorio `clientWidth=scrollWidth=1350`. | [Móvil 375×812](evidencia/FE12_OFERTA_movil.jpg) · [Escritorio 1350×890](evidencia/FE12_OFERTA_escritorio.jpg) |
| `/panel/espacios/:spaceId/calendario` · anfitrión | Fixture de espacio `Espacio FE-11 de reserva sintética`: zona `America/Santiago`, horario semanal 09:00–17:00, sin bloqueos próximos. Los avisos explican validación de horas DST. | Tab conserva foco visible de 3 px. Contraste: 0 fallos en 73 textos móvil y 79 escritorio. Móvil sin overflow horizontal; escritorio sin overflow. | [Móvil 375×812](evidencia/FE12_CALENDARIO_host_movil.jpg) · [Escritorio 1350×890](evidencia/FE12_CALENDARIO_host_escritorio.jpg) |
| `/mi-cuenta/verificaciones` · anfitrión | Go confirma KYC y KYB aprobados/elegibles, historial y fixture PNG sintético privado `synthetic-png-v1`; no se iniciaron casos ni se generaron archivos durante esta pasada. | Navegación por Tab llega al selector de tipo con anillo de 3 px. Contraste: 0 fallos en 42 textos móvil y 48 escritorio. Viewports móvil 390×844 y escritorio 1365×900, sin overflow horizontal. | [Móvil 375×812](evidencia/FE12_KYC_anfitrion_movil.jpg) · [Escritorio 1350×890](evidencia/FE12_KYC_anfitrion_escritorio.jpg) |
| `/reservas/:id` · anfitrión participante | Fixture Go de reserva finalizada: contrato firmado por ambos, historial de check-in/check-out/recepción y revisión sintética; conversación vacía, de solo lectura. El contenido largo y los estados se presentan como texto. | Se usó Tab para comprobar el foco visible de 3 px. Contraste del detalle largo: 0 fallos en el muestreo de 85 textos; sin overflow horizontal en móvil/escritorio. | Contrato/operación: [móvil 375×812](evidencia/FE12_RESERVA_CONTRATO_movil.jpg) · [escritorio 1350×890](evidencia/FE12_RESERVA_CONTRATO_escritorio.jpg). Conversación: [móvil 375×812](evidencia/FE12_RESERVA_CONVERSACION_movil.jpg) · [escritorio 1350×890](evidencia/FE12_RESERVA_CONVERSACION_escritorio.jpg) |
| `/reservas/:id` · anfitrión participante · fallo terminal | Fixture local identificado en `.local/testing/fe11-angular-rejection-reservation.txt`: Go confirma `Cancelada por firma`; no hay operaciones de llegada ni una conversación editable. | Estado terminal en modo de solo lectura. Tab deja foco visible de 3 px. Contraste: 0 fallos en 50 textos móvil y 56 escritorio; sin overflow horizontal. | [Móvil 375×812](evidencia/FE12_RESERVA_RECHAZO_movil.jpg) · [Escritorio 1350×890](evidencia/FE12_RESERVA_RECHAZO_escritorio.jpg) |
| `/administracion` · administrador | Go devolvió cola KYC vacía; el mensaje comunica que no hay registros. Las secciones administrativas y filtros permanecen disponibles. | Tab enfoca «Cola KYC» con anillo de 3 px. Contraste: 0 fallos en 29 textos móvil y 36 escritorio. A 390 px la tira de secciones se desplaza horizontalmente dentro de su propio carril; la página no desborda (`375=375`). | [Móvil 375×812](evidencia/FE12_ADMIN_QUEUE_movil.jpg) · [Escritorio 1350×890](evidencia/FE12_ADMIN_QUEUE_escritorio.jpg) |
| `/administracion/disputas` · administrador | Fixture sintético de reserva leído desde Go: garantía `autorizada`, CLP 50.000 autorizados, 0 capturados y 0 liberados; sin liquidación ni comprobante. No se ejecutaron operaciones. | Tab enfoca el selector de resultado con anillo de 3 px. Contraste: 0 fallos en 29 textos móvil y 36 escritorio. Sin overflow horizontal en ambas vistas. | [Móvil 375×812](evidencia/FE12_FINANZAS_admin_movil.jpg) · [Escritorio 1350×890](evidencia/FE12_FINANZAS_admin_escritorio.jpg) |
| Error y offline | El caso de fallo terminal anterior es un estado real confirmado por Go. La desconexión se cubre con la regresión existente `announces offline state and refreshes readiness when connectivity returns` en `frontend/src/app/app.spec.ts`; readiness se mantuvo saludable y no se cortó la red durante la sesión visual. | No se presenta una captura offline como si fuera una caída real. Axe-core y la regresión de conectividad complementan la inspección visual; el muestreo manual no certifica WCAG. | No se generó una captura offline porque no se alteró el estado de red del navegador ni se interrumpió Go. |

Las capturas usan viewport de navegador 390×844 CSS px y 1365×900 CSS px. El JPEG móvil mide 375×812 por el scrollbar reservado de 15 px; el JPEG de escritorio mide 1350×890. El anillo azul `rgb(29,78,216)` de 3 px da 6,28:1 sobre Crema. Los fixtures/cuentas son locales y sintéticos; sus credenciales permanecen fuera del PR.

## FE-13 — Stack Angular Docker y runner sin mock (aceptado y fusionado)

| Superficie | Validación | Evidencia |
| --- | --- | --- |
| Instalación reproducible | `npm ci`: 403 paquetes, auditoría 0 vulnerabilidades. Imagen Node 24.21.0 Alpine fijada por digest; npm 12.1.0 y package-lock | Pasó; imagen Angular `sha256:2b0b71689b4be10667e485afc6fd550fa7bb1fc3da5aa3caabcae1059eac72b6` |
| Angular/Go/Compose | Runner integrado comprueba shell SSR desde URL host y red Compose, config pública `127.0.0.1`, readiness Go HTTP 200, CORS y preflight login HTTP 204; no depende del mock | Pasó con proyecto `espacigo_fe13_20261012203806_986313`; sin mock activo |
| Aislamiento/reinicio | Inspección Docker: Angular solo en red `_api`, sin variables de credenciales DB. API se detiene; Angular root y healthz permanecen HTTP 200; se reinicia el servicio backend/migración | Pasó. Redes/subredes temporales 10.240.0.0/24–10.240.2.0/24; volumen exclusivo eliminado al cerrar |
| API caída/restaurada | Readiness host deja de conectar al detener Go; tras `up-frontend` el runner vuelve a obtener readiness HTTP 200/CORS/preflight exitosos | Pasó, stack/volumen/redes del proyecto temporal retirados |
| Sesión | Regresión de componente con `IdentitySession` real: error de red en readiness mantiene `status=authenticated`, Bearer y sesión; readiness recuperado no cambia la sesión | Pasó en Vitest; no se ejecutó un login visual live durante el reinicio |

**Regresión Angular:** `npm test -- --watch=false`, 169 pruebas/28 archivos; `npm run api:check` pasó con `ng serve` detenido; `npm run build` prerenderizó 3 rutas. **Stack Docker:** `scripts/test-frontend-docker.sh` pasó con puertos API 18680/Angular 18683 y runner sin credenciales; imagen runner `sha256:83812f89384f93e01a5520122c4b87bd3bb97964a569476a7207c72f129308e8`. Transcripción de criterios/alcance: [`evidencia/FE13_docker_stack.log`](evidencia/FE13_docker_stack.log). No se usó ningún volumen persistente y la limpieza confirmó cero contenedores/redes/volúmenes con prefijo FE-13. La pérdida de conexión no revoca un token; si Go responde 401 vigente, se conservan las reglas existentes de expiración. FE-14 se ejecuta a continuación; FE-15 queda pendiente de aceptación explícita.

Origen base: `origin/main` tras merge #287 (`106942ff`); código validado en `c6b46db9bb62ed7ba0ab943e1398e32b7645c450`, rama `codex/frontend-local-1-fe13-docker`. PR #288 abierto por `HernanMEC`, revisión solicitada a `HernanEspinozaDev`. El script `scripts/test-frontend-docker.sh` crea un proyecto y volumen nuevos con puertos propios, secretos/directorios fuera de datos locales y limpieza acotada a esos recursos. La instalación/ejecución no incluye credenciales ni cambia configuración privada. FE-13 aceptado y fusionado en #288. FE-14 se prepara en el corte actual; FE-15 (retirar el mock) requiere aceptación explícita de FE-14.


## FE-14 — Regresión consolidada y matriz completa (aceptado explícitamente)

**Base de aceptación:** `main` con #289 y PR #290 aprobado/fusionado, merge `b94dba37d80fa63c92d3e8aedffe8d6740ce2e0d`. **Source commit E2E:** `6277140b8fef7226fe29d10284b67d36d943d780`. La base de planificación tiene 118 paths/144 operaciones; este seguimiento agrega una consulta GET read-only de hora lógica en OpenAPI, por lo que la revisión actual contiene 119 paths/145 operaciones. Las operaciones de negocio existentes siguen mapeadas a UI o excepción técnica; el helper de reloj no modifica el modelo de dominio. Los estados de FE-04 y FE-11 se actualizaron a aceptados/fusionados, y FE-13 a #288. La matriz registra el estado completo, no afirma que todas las operaciones se hayan repetido en una sola prueba E2E.

| Recorrido FE-14 | Comprobación real | Resultado | Evidencia |
| --- | --- | --- | --- |
| Arrendatario, anfitrión, administrador y tercero | Login/session Go; guardias y navegación; el tercero no abre vistas anfitrión/admin. Roles host/admin se preparan solo en la DB descartable y Go los devuelve al consultar sesión. | 4 cuentas `.invalid`; almacenamiento local/session vacío para Bearer; pasa | [Móvil arrendatario 390×844](evidencia/fe14_renter_mobile.jpg) · [escritorio](evidencia/fe14_renter_desktop.jpg); [móvil anfitrión](evidencia/fe14_host_mobile.jpg) · [escritorio](evidencia/fe14_host_desktop.jpg); [móvil admin](evidencia/fe14_admin_mobile.jpg) · [escritorio](evidencia/fe14_admin_desktop.jpg) |
| Hora lógica y fechas locales | `GET /local/booking-trial/business-clock`; Bearer requerido; normal=`time.Now`, demo FE-14 aislada solamente. | Go actualiza el valor al cambiar el archivo de reloj sin reiniciar; Angular calcula la fecha de “hoy” en el IANA del espacio. Configuración no admite paths fuera del archivo dedicado. | [“Hoy” móvil 390×2073](evidencia/aceptacion-fe14-b94dba37/fe14_manual_clock_today_mobile.jpg) |
| Exportación JSON/ZIP privada | Descarga por interfaz con Bearer; JSON propio sin token/contraseña/otra cuenta; firma `PK`, `manifest.json` y `data.json` comprobados del ZIP. | Pasa contra Go; backend logs sin credenciales | Capturas de privacidad incluidas en [recorrido arrendatario](evidencia/fe14_renter_desktop.jpg) |
| Caída/restauración de Go | Espera el `requestfailed` autenticado de elegibilidad, error visible y tarjetas vacías. Tras restaurar, compara UUID/rol, conserva login único, retrasa el request para observar spinner/vacío y luego exige HTTP 200 Bearer, payload Go y dos tarjetas renderizadas. | Misma sesión/cuenta sin login nuevo; control de respuesta demorada bloquea falso positivo; pasa | [sesión restaurada 1365×900](evidencia/fe14_restored_session_desktop.jpg) |
| Aislamiento del runner | Montajes limitados a accounts.json read-only y artefactos/resultados read-write; comprobación de paths privados/secretos y env ausentes. | Solo 3 entradas `/run/fe14`; secretos, evidencia y replay no existen/no son legibles; pasa | [manifiesto de evidencia](evidencia/FE14_aceptacion.md) |
| Responsive/foco | Playwright Chromium 390×844 y 1365×900; screenshots full-page y JSON con medidas viewport/documento; Tab visible en enlace/heading. | Dimensiones exactas, overflow horizontal ausente en vistas evaluadas; ver capturas JPEG | [manifiesto de evidencia](evidencia/FE14_aceptacion.md) |

**Regresión consolidada (reloj de negocio manual):** Angular 180 pruebas/32 archivos; Go `go test ./internal/platform/businessclock ./internal/adapters/fakebooking ./internal/booking/transport/http ./internal/booking ./internal/contract ./internal/operation ./internal/dispute ./internal/damageclaim` pasó; `api:check` pasó con `ng serve` detenido; build/prerender pasó (3 rutas). El overlay de reloj se carga solo por el stack FE-14 aislado; el runtime normal mantiene reloj real. El comando local fija RFC3339 y avanza duraciones sin cambiar hora del host. Go confirmó check-in 201 a 2026-10-13 02:00 America/Santiago, check-out 201 y recepción 201 a las 03:00; liquidación prematura 409; en 2026-10-14 03:00 la conciliación posterior a operación incierta respondió 200 y Go confirmó liquidación de CLP 12.000. El endpoint lógico actualizado sin reinicio y Angular mostró la fecha local 14-10. Capturas, SHA-256, medidas y límites quedan en [`evidencia/FE14_aceptacion.md`](evidencia/FE14_aceptacion.md). Stack `espacigo_fe14_demo_18a028644c87`, volumen exclusivo; sin intervención del volumen persistente. FE-14 fue aceptado explícitamente por el usuario sobre `main` `d02f9c8` con PR #290/#291 fusionados; FE-15 comienza en esta rama.

**Paridad y aceptación:** matriz completa en términos de operación; FE-02–FE-13 aceptados/fusionados; FE-14 aceptado explícitamente por el usuario el 2026-10-13 tras revisar evidencia, con #290 y #291 fusionados. FE-14 valida invariantes transversales sin afirmar que repite cada combinación Go. No se simularon datos productivos ni se introdujeron credenciales. La aceptación autoriza iniciar FE-15.

## Adecuación arquitectónica previa a la aceptación del usuario

Nota histórica previa a la aceptación explícita: el merge técnico FE-14 por sí solo no cerraba la aceptación arquitectónica. El [PR #290](https://github.com/HernanEspinozaDev/espaciGo/pull/290) separa vistas/fachadas/puertos/adaptadores, introduce núcleo puro, formularios tipados, UI compartida y gates estrictos de dependencias/estilos. [Auditoría](revision_arquitectura.md), [manifiesto del código probado `70a97c4`](evidencia/arquitectura/manifest.json) y [recorrido preparado](demo_aceptacion_arquitectura.md). Validación: 178 pruebas Angular, lint/arquitectura, contrato, build/prerender, Docker aislado y 4 E2E autenticadas. Las capturas de esta repetición no sustituyen las históricas; la la aceptación explícita FE-14 fue registrada posteriormente en `main` `d02f9c8`, tras fusionar #291.

La revisión de #290 detectó dos brechas de control en `f461c2e`; se corrigieron con namespaces derivados de OpenAPI/YAML y controles de métricas/breakpoints sin excepciones por feature. Gate: 28 pruebas del tooling, 178 Angular, contrato/build y E2E 4/4 posteriores. [Evidencia](evidencia/arquitectura/controles/manifest.json). Los 20 estilos afectados compilan a CSS idéntico al baseline; no se hizo un rediseño. La aceptación del usuario de FE-14 sigue pendiente de demo sobre el merge aprobado.

**Demo sobre el merge #290 (inventario inicial supersedido):** el estado de borrador/KYC pendiente corresponde a la preparación anterior al recorrido. El inventario vigente, sin credenciales, está en `.local/testing/fe14-demo/fixture-manifest.md`; ambas verificaciones fueron aprobadas, Go confirma elegibilidad, el espacio está publicado y la reserva sigue `lista_para_checkin`.


### FE-14 — Transacciones de aceptación en Go (2026-10-13)

| Escenario | Resultado |
| --- | --- |
| KYC y publicación elegible | KYC de anfitrión/arrendatario aprobado por el panel admin; Go confirmó elegibilidad. Guardado tarifa horaria CLP 12.000, zona IANA, y publicación del espacio por el flujo del dueño. Búsqueda/cotización desde Angular con respuestas Go exitosas. |
| Reserva y pago fake | `POST` reserva HTTP 201, pago fake HTTP 200. Garantía: el POST devolvió 202 (operación aceptada/confirmada) y la lectura Go confirmó `autorizada`; aprobación de anfitrión HTTP 200. |
| Contrato/PDF | Firmas de anfitrión y arrendatario, estado Go `lista_para_checkin`, descarga PDF privado válida. |
| Exportaciones/permisos | JSON/ZIP del titular validados y tercero denegado. |
| Operación/finanzas | Check-in Go HTTP 409 porque aún no es el día IANA de inicio; no se alteró reloj/fecha. Check-out/recepción pendientes. `TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay` pasó con PostgreSQL aislado y reloj sustituible, incluida liquidación/cierre/comprobante tras +24 h; la reserva viva no se presenta como liquidada. |

Capturas JPEG/medidas/hashes y artefactos están en [evidencia/FE14_aceptacion.md](evidencia/FE14_aceptacion.md), incluida [reserva lista / check-in 409](evidencia/aceptacion-fe14-b94dba37/fe14_acceptance_checkin_waiting_for_iana_day_mobile.jpg). La revisión adicional se solicitó y se fusionó en [PR #291](https://github.com/HernanEspinozaDev/espaciGo/pull/291). La primera ausencia de check-in/check-out fue una condición temporal impuesta por Go y quedó resuelta mediante reloj de negocio aislado; el usuario aceptó FE-14 el 2026-10-13 sobre `main` `d02f9c8`, lo que habilita el corte FE-15.


## FE-15 — Retirada del mock ([PR #292 en revisión](https://github.com/HernanEspinozaDev/espaciGo/pull/292))

Base: FE-14 aceptado explícitamente por el usuario el 2026-10-13, con #290/#291 fusionados; `main` `d02f9c81c35b4979fd20d8b027783ab00e1d1ab4`. Angular es el servicio frontend normal del Compose y la guía describe el flujo Angular como predeterminado.

| Elemento retirado/migrado | Reemplazo / comprobación | Estado en este PR |
| --- | --- | --- |
| `cmd/mock`, `internal/mockserver`, cliente y pruebas legacy `mock/` | Angular CSR/SSR, facades y pruebas existentes; Playwright autenticado contra Go | Eliminado; suite Angular 180/180 |
| Target Docker `mock`, compilación TypeScript legacy y servicio Compose de mock | `angular-frontend` incluido en `compose.yaml`; build independiente `frontend/Dockerfile`; runner solo con overlay | Eliminado; Compose/build/runner pasaron |
| Referencias de origen CORS/health al mock | Origen Angular `localhost`/`127.0.0.1`; `scripts/verify-http.py` comprueba shell/config/readiness/CORS | Actualizado y verificado |
| Scripts QA, aislamiento y stack aislado | QA Angular + backend sin Node; red aislada para `angular-frontend`; runner sin secretos/DB | Migrado; Docker/QA pasaron |
| Enlace de invitación local heredado | Angular `/reservas`; Go sigue comprobando participación al consultar | Actualizado; suite Go pasó |

La suite legacy no es el criterio de paridad: cada capacidad funcional de UI conserva pruebas Angular o integración/E2E Go trazadas arriba. No se borran rutas ni políticas Go; esta retirada no modifica proveedores, GCP ni funciones futuras. FE-15 validado y publicado en PR #292, autor HernanMEC, revisión solicitada a HernanEspinozaDev; rama `codex/frontend-fe15-mock-retirement`, desde `main` `d02f9c8`; commit inicial de implementación `b9af256b0c920efdcce8d6ebffe2675af522a854` (este commit actualiza la referencia de PR). Angular: 180 pruebas/32 archivos; lint/arquitectura: 13+15 gates; `api:check`; build/prerender (3 rutas). Go: `go test ./...`, `go vet ./...`, 89 integraciones obligatorias/127 eventos y lifecycle/retry 5x. Docker QA #01 y el stack FE-15 pasaron. Stack `espacigo_fe15_20261013042837_427893`, puertos API 19200/Mailpit 19201/Angular 19202; Angular siguió respondiendo durante la caída de Go y volvió readiness al restaurarlo. Imagen Angular `sha256:f5e3a9fa87bd26d5c22c90779d47536a372830e3e80f1ff53154335d8a3a13bd`; recursos desechables retirados. No se tocó volumen persistente ni se inició GCP/proveedores reales. Se creó `scripts/test-local-qa-backend.sh` para que Go QA no requiera runtime Node; `api:generate` normaliza salida para dejar limpio `api:check`.
