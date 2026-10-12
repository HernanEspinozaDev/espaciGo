# Matriz de paridad del mock hacia Angular

Fecha: 2026-10-12. Snapshot OpenAPI 3.1.0 de `b0f62b976c9586d260cea15ed9acc29ab12257a7`: **118 paths / 144 operaciones**. Inventario de contrato; la evidencia y el estado FE-02 se registran abajo. Los demás estados Pendiente no implican cobertura.

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
| FE-10 | 21 | Implementado y corrigiendo comentarios; [PR #285 en revisión](https://github.com/HernanEspinozaDev/espaciGo/pull/285) |
| FE-11 | 29 | Pendiente |
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
| `GET /admin/local/review-reports` | `listPendingReviewReports` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `POST /admin/local/review-reports/{reportID}/{action}` | `moderateReviewReport` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/local/notices` | `listTerminalLocalNotices` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `POST /admin/local/notices/{noticeID}/reopen` | `reopenLocalNotice` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /auth/terms` | `listLocalTerms` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/register` | `registerAccount` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/verification/reissue` | `reissueVerification` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/verification` | `verifyEmail` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/password/recovery` | `requestPasswordRecovery` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/password/recovery/consume` | `consumePasswordRecovery` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /auth/password/change` | `changePassword` | Bearer; permisos en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `GET /profile` | `getOwnProfile` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; PR en revisión |
| `PUT /profile` | `updateOwnProfile` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; PR en revisión |
| `GET /profile/photo` | `getOwnSyntheticProfilePhoto` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; PR en revisión |
| `PUT /profile/photo` | `generateOwnSyntheticProfilePhoto` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; PR en revisión |
| `DELETE /profile/photo` | `removeOwnSyntheticProfilePhoto` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; PR en revisión |
| `GET /profile/photo/content` | `getOwnSyntheticProfilePhotoContent` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; PR en revisión |
| `GET /payout-account` | `getOwnSyntheticPayoutAccount` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; PR en revisión |
| `PUT /payout-account` | `createOrReplaceOwnSyntheticPayoutAccount` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; PR en revisión |
| `DELETE /payout-account` | `revokeOwnSyntheticPayoutAccount` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; PR en revisión |
| `GET /rights-requests` | `listOwnRightsRequests` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; PR en revisión |
| `POST /rights-requests` | `requestDataRight` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Implementado; PR en revisión |
| `GET /privacy/export` | `exportOwnIdentityData` | Bearer; permisos en Go | FE-04 — Exportación del titular | Implementado; PR en revisión |
| `GET /privacy/export/archive` | `downloadCompleteOwnDataArchive` | Bearer; permisos en Go | FE-04 — Exportación del titular | Implementado; PR en revisión |
| `POST /privacy/suppression-requests/{request_id}/review` | `reviewLocalSuppressionBlockers` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Pendiente |
| `POST /privacy/suppression-requests/{request_id}/execute` | `executeLocalSyntheticSuppression` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Pendiente |
| `POST /privacy/retention/purge` | `purgeExpiredLocalPrivacyData` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Pendiente |
| `GET /admin/credential-notices` | `listTerminalLocalCredentialNotices` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `POST /admin/credential-notices/{event_id}/reopen` | `reopenTerminalLocalCredentialNotice` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /privacy/suppression-requests` | `listPendingLocalSuppressionRequests` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Pendiente |
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
| `GET /admin/verifications/{id}/evidence/{evidenceId}` | `reviewSyntheticEvidenceContent` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `DELETE /admin/verifications/{id}/evidence/{evidenceId}` | `deleteReviewedSyntheticEvidence` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/verifications/{id}/evidence` | `listEvidenceForReview` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/verifications` | `listPendingVerifications` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/verifications/failed` | `listRejectedVerifications` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `POST /admin/verifications/{id}/review` | `reviewSyntheticVerification` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `POST /admin/verifications/{id}/revoke` | `revokeSyntheticEligibility` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
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
| `GET /local/booking-trial/reservations/{reservationId}/settlement` | `getOwnLocalSettlement` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 en revisión |
| `POST /local/booking-trial/reservations/{reservationId}/settlement` | `requestLocalFakeSettlement` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 en revisión |
| `POST /local/booking-trial/reservations/{reservationId}/settlement-reconcile` | `reconcileLocalFakeSettlement` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 en revisión |
| `GET /local/booking-trial/reservations/{reservationId}/receipt` | `getOwnLocalSettlementReceipt` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 en revisión |
| `GET /local/booking-trial/reservations/{reservationId}/guarantee` | `getOwnSyntheticReservationGuarantee` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 en revisión |
| `POST /local/booking-trial/reservations/{reservationId}/guarantee` | `executeLocalSyntheticGuaranteeOperation` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 en revisión |
| `POST /local/booking-trial/reservations/{reservationId}/guarantee-reconcile` | `reconcileLocalGuaranteeFakeResult` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 en revisión |
| `POST /local/booking-trial/reservations/{reservationId}/financial-decision` | `recordAdminLocalFinancialDecision` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Implementado; PR #285 en revisión |
| `POST /local/booking-trial/payment-events` | `receiveAuthenticatedLocalPaymentEvent` | HMAC Backend | Técnica — Callback HMAC; no UI ni secreto en Angular | Sin UI directa justificada |
| `POST /local/booking-trial/reservations/{reservationId}/decision` | `decideLocalTrialReservation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `GET /local/booking-trial/reservations/{reservationId}/cancellation-preview` | `previewLocalTrialReservationCancellation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `POST /local/booking-trial/reservations/{reservationId}/cancel` | `cancelLocalTrialReservation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `POST /local/booking-trial/reservations/{reservationId}/refund` | `simulateLocalTrialRefund` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Implementado; PR #282 |
| `GET /local/booking-trial/reservations/{reservationId}/messages` | `listOwnLocalReservationMessages` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Implementado; 38 mensajes, páginas 30 + anteriores y tercero denegado |
| `POST /local/booking-trial/reservations/{reservationId}/messages` | `sendOwnLocalReservationMessage` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Implementado; estado editable y terminal denegado por Go |
| `POST /local/booking-trial/reservations/{reservationId}/messages/read` | `advanceOwnLocalReservationMessageReadCursor` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Implementado; cursor y contador confirmados por Go |
| `GET /local/booking-trial/reservations/{reservation_id}/disputes` | `listOwnReservationTrialDisputes` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 en revisión |
| `POST /local/booking-trial/reservations/{reservation_id}/disputes` | `openOwnHostedTrialDispute` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 en revisión |
| `GET /local/booking-trial/disputes/{dispute_id}/history` | `getOwnTrialDisputeHistory` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 en revisión |
| `GET /admin/disputes` | `listOpenLocalTrialDisputes` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 en revisión |
| `POST /admin/disputes/{dispute_id}/close` | `closeLocalTrialDispute` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 en revisión |
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
| `POST /local/booking-trial/reservations/{reservationID}/damage-claim/defense/evidence` | `createSyntheticDamageClaimDefenseEvidence` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 en revisión |
| `GET /local/booking-trial/reservations/{reservationID}/damage-claim` | `getOwnSyntheticDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 en revisión |
| `POST /local/booking-trial/reservations/{reservationID}/damage-claim` | `openSyntheticHostDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 en revisión |
| `POST /local/booking-trial/reservations/{reservationID}/damage-claim/defense` | `submitSyntheticDamageClaimDefense` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Implementado; PR #285 en revisión |
| `GET /admin/local/audit-events` | `listAdminLocalAuditEvents` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/local/audit-events/export` | `exportAdminLocalAuditEvents` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/local/accounts/{accountID}` | `getAdminLocalAccountByID` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/local/published-spaces` | `listAdminLocalPublishedSpaces` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/local/published-spaces/{spaceID}` | `getAdminLocalPublishedSpace` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `POST /admin/local/accounts/{accountID}/block` | `blockLocalAccount` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `POST /admin/local/accounts/{accountID}/unblock` | `unblockLocalAccount` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/local/reports/reservations` | `getAdminLocalReservationsReport` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/local/reports/finance` | `getAdminLocalFinanceReport` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/local/reports/disputes` | `getAdminLocalDisputesReport` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/local/reservations` | `listAdminLocalReservations` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/local/reservations/{reservationID}` | `getAdminLocalReservation` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/local/damage-claims` | `listOpenLocalSyntheticDamageClaims` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 en revisión |
| `GET /admin/local/damage-claims/{claimID}` | `getAdminLocalSyntheticDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 en revisión |
| `GET /admin/local/damage-claims/{claimID}/evidence/{evidenceID}` | `getAdminLocalDamageClaimEvidence` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 en revisión |
| `POST /admin/local/damage-claims/{claimID}/resolution` | `resolveLocalSyntheticDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Implementado; PR #285 en revisión |

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

FE-09 aprobado y fusionado en PR #284, merge `0d3bac8a06457899239a49274b54ad6807846f42`. FE-10 está en revisión en PR #285; FE-11 espera aprobación y fusión de FE-10.

**Aceptación:** FE-09 aprobado y fusionado en PR #284. FE-10 está en revisión; FE-11 queda pendiente hasta su aprobación y fusión.

## Trazabilidad FE-10 — Disputas y finanzas ([PR #285 en revisión](https://github.com/HernanEspinozaDev/espaciGo/pull/285))

| Flujo/operación Go | Interfaz Angular | Prueba/evidencia | Estado/límite |
| --- | --- | --- | --- |
| Incidencias e historial; reclamo, descargo y evidencia sintética | Detalle de reserva → Disputas y finanzas | Go desechable: anfitrión abre incidencia/reclamo; arrendatario envía descargo; tercero obtiene 404; administrador cierra incidencia y reclamo | Implementado; evidencia solo privada PNG sintética |
| Evidencia privada de participantes y administración | Controles de descarga con Bearer y Blob temporal | La descarga autenticada se verificó; logout dejó cero imágenes privadas en DOM y revocó el Blob URL | Implementado; sin archivos reales/arbitrarios |
| Garantía, deducción y conciliación | Panel de reserva y cola administrativa | Go desechable confirmó decisión de CLP 1.000; integración con reloj sustituible completó autorización, captura, liberación, conciliación y replay | Implementado; no hay movimiento monetario |
| Liquidación y comprobante | Panel de reserva y conciliación administrativa | Test Go con reloj sustituible cruzó check-out + 24 h, confirmó liquidación, reconciliación, transición terminal y comprobante único | Éxito probado en base PostgreSQL/PostGIS temporal; el fixture del navegador conserva sus fechas/estado originales |

FE-09 aceptado y fusionado en PR #284, merge `0d3bac8a06457899239a49274b54ad6807846f42`. Angular 4200 HTTP 200 y Go 18280 `/health/ready` HTTP 200. Stack desechable `espacigo_fe10_20261012` con volumen nuevo; anfitrión/arrendatario participantes y admin aceptados por Go; tercero denegado 404 para disputas/reclamos/garantía. La incidencia se abrió y cerró; reclamo y descargo con evidencia PNG sintética se consultaron con Bearer; admin resolvió y Go confirmó decisión acogida con deducción de 1.000 CLP, evidencia y estado `pendiente`; sin dinero real ni captura aplicada. Comprobante GET 404; el POST settlement recibió HTTP 404 para esta reserva. El caso sigue en `en_disputa` con decisión financiera pendiente y además aún no cumple las 24 h desde check-out, así que no se intenta forzar una liquidación. No se forzó tiempo y no se tocó volumen persistente.

Evidencia Angular conectada a Go: [participante escritorio 1425×2787](evidencia/FE10_disputas_escritorio.jpg), [participante móvil 375×4050](evidencia/FE10_disputas_movil.jpg), [cola admin escritorio 1425×2289](evidencia/FE10_admin_escritorio.jpg), [cola admin móvil 375×2703](evidencia/FE10_admin_movil.jpg), [panel actual desktop 1265×712](evidencia/FE10_admin_actual_escritorio.jpg) y [móvil 375×812](evidencia/FE10_admin_actual_movil.jpg). Las capturas de panel actual son viewports; las primeras son páginas completas.

Correcciones de revisión: commit `b2da3243becb9db0fa10828baf5cae9782af474f`. Regresión Angular: 149 pruebas en 26 archivos, incluidas selección admin A→B con GET tardío, sesión admin A→B con intento incierto, aislamiento por cuenta y respuestas tardías exitosas/4xx sin borrar la instancia nueva. Test Go: `go test ./internal/adapters/postgres/booking -count=1` en PostgreSQL/PostGIS descartable, con reloj inyectado; tres pruebas de integración pasan y verifican garantía, liquidación/reconciliación y comprobante exitosos. `api:check` pasó con `ng serve` detenido; build/prerender pasó con 3 rutas. FE-11 sigue pendiente hasta aprobar y fusionar FE-10.
