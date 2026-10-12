# Matriz de paridad del mock hacia Angular

Fecha: 2026-10-11. Snapshot OpenAPI 3.1.0 de `b0f62b976c9586d260cea15ed9acc29ab12257a7`: **118 paths / 144 operaciones**. Inventario de contrato; la evidencia y el estado FE-02 se registran abajo. Los demás estados Pendiente no implican cobertura.

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
| FE-03 | 7 | Implementado; PR en revisión |
| FE-04 | 13 | Pendiente |
| FE-05 | 10 | Pendiente |
| FE-06 | 26 | Pendiente |
| FE-07 | 8 | Pendiente |
| FE-08 | 10 | Pendiente |
| FE-09 | 8 | Pendiente |
| FE-10 | 21 | Pendiente |
| FE-11 | 29 | Pendiente |
| Técnica | 1 | No requiere pantalla directa |

La cobertura completa requiere FE-01/12–15 para diseño, Docker, calidad y retirada, aunque no agreguen operaciones de API.

## Inventario por operación

| Método y ruta relativa a `/api/v1` | operationId | Acceso declarado | Corte / pantalla | Estado |
| --- | --- | --- | --- | --- |
| `GET /spaces/{spaceID}/reviews` | `listActiveSpaceReviews` | Sin Bearer declarado; reglas en Go | FE-09 — Reseñas visibles autorizadas | Pendiente |
| `GET /local/booking-trial/reservations/{reservationID}/reviews` | `listReservationReviews` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Pendiente |
| `POST /local/booking-trial/reservations/{reservationID}/reviews` | `createReservationReview` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Pendiente |
| `POST /local/booking-trial/reservations/{reservationID}/reviews/{reviewID}/report` | `reportSpaceReview` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Pendiente |
| `GET /local/reputation/me` | `getOwnRenterReputation` | Bearer; permisos en Go | FE-09 — Reputación propia | Pendiente |
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
| `GET /profile` | `getOwnProfile` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Pendiente |
| `PUT /profile` | `updateOwnProfile` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Pendiente |
| `GET /profile/photo` | `getOwnSyntheticProfilePhoto` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Pendiente |
| `PUT /profile/photo` | `generateOwnSyntheticProfilePhoto` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Pendiente |
| `DELETE /profile/photo` | `removeOwnSyntheticProfilePhoto` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Pendiente |
| `GET /profile/photo/content` | `getOwnSyntheticProfilePhotoContent` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Pendiente |
| `GET /payout-account` | `getOwnSyntheticPayoutAccount` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Pendiente |
| `PUT /payout-account` | `createOrReplaceOwnSyntheticPayoutAccount` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Pendiente |
| `DELETE /payout-account` | `revokeOwnSyntheticPayoutAccount` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Pendiente |
| `GET /rights-requests` | `listOwnRightsRequests` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Pendiente |
| `POST /rights-requests` | `requestDataRight` | Bearer; permisos en Go | FE-04 — Cuenta y privacidad | Pendiente |
| `GET /privacy/export` | `exportOwnIdentityData` | Bearer; permisos en Go | FE-04 — Exportación del titular | Pendiente |
| `GET /privacy/export/archive` | `downloadCompleteOwnDataArchive` | Bearer; permisos en Go | FE-04 — Exportación del titular | Pendiente |
| `POST /privacy/suppression-requests/{request_id}/review` | `reviewLocalSuppressionBlockers` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Pendiente |
| `POST /privacy/suppression-requests/{request_id}/execute` | `executeLocalSyntheticSuppression` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Pendiente |
| `POST /privacy/retention/purge` | `purgeExpiredLocalPrivacyData` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Pendiente |
| `GET /admin/credential-notices` | `listTerminalLocalCredentialNotices` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `POST /admin/credential-notices/{event_id}/reopen` | `reopenTerminalLocalCredentialNotice` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /privacy/suppression-requests` | `listPendingLocalSuppressionRequests` | Bearer; permisos en Go | FE-11 — Administración de privacidad; operación explícita | Pendiente |
| `GET /verifications` | `listOwnVerifications` | Bearer; permisos en Go | FE-05 — Verificación sintética y elegibilidad | Pendiente |
| `POST /verifications` | `startSyntheticVerification` | Bearer; permisos en Go | FE-05 — Verificación sintética y elegibilidad | Pendiente |
| `GET /verifications/eligibility` | `listOwnSyntheticEligibility` | Bearer; permisos en Go | FE-05 — Verificación sintética y elegibilidad | Pendiente |
| `GET /verifications/{id}` | `getOwnVerification` | Bearer; permisos en Go | FE-05 — Verificación sintética y elegibilidad | Pendiente |
| `GET /verifications/{id}/history` | `listOwnVerificationHistory` | Bearer; permisos en Go | FE-05 — Verificación sintética y elegibilidad | Pendiente |
| `POST /verifications/{id}/retry` | `retryRejectedVerification` | Bearer; permisos en Go | FE-05 — Verificación sintética y elegibilidad | Pendiente |
| `GET /verifications/{id}/evidence` | `listOwnSyntheticEvidence` | Bearer; permisos en Go | FE-05 — Verificación sintética y elegibilidad | Pendiente |
| `POST /verifications/{id}/evidence` | `uploadSyntheticEvidence` | Bearer; permisos en Go | FE-05 — Verificación sintética y elegibilidad | Pendiente |
| `GET /verifications/{id}/evidence/{evidenceId}` | `getOwnSyntheticEvidenceContent` | Bearer; permisos en Go | FE-05 — Verificación sintética y elegibilidad | Pendiente |
| `DELETE /verifications/{id}/evidence/{evidenceId}` | `deleteOwnSyntheticEvidence` | Bearer; permisos en Go | FE-05 — Verificación sintética y elegibilidad | Pendiente |
| `GET /admin/verifications/{id}/evidence/{evidenceId}` | `reviewSyntheticEvidenceContent` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `DELETE /admin/verifications/{id}/evidence/{evidenceId}` | `deleteReviewedSyntheticEvidence` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/verifications/{id}/evidence` | `listEvidenceForReview` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/verifications` | `listPendingVerifications` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `GET /admin/verifications/failed` | `listRejectedVerifications` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `POST /admin/verifications/{id}/review` | `reviewSyntheticVerification` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `POST /admin/verifications/{id}/revoke` | `revokeSyntheticEligibility` | Bearer; permisos en Go | FE-11 — Administración y colas | Pendiente |
| `POST /auth/login` | `login` | Sin Bearer declarado; reglas en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `GET /auth/session` | `currentSession` | Bearer; permisos en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `GET /spaces/categories` | `listSpaceCategories` | Bearer; permisos en Go | FE-03 — Catálogo de categorías y perfiles | Implementado en FE-03; PR en revisión |
| `GET /spaces/categories/{categoryCode}/attributes` | `getSpaceAttributeProfile` | Bearer; permisos en Go | FE-03 — Catálogo de categorías y perfiles | Implementado en FE-03; PR en revisión |
| `GET /spaces/categories/{categoryCode}/attributes/{version}` | `getVersionedSpaceAttributeProfile` | Bearer; permisos en Go | FE-03 — Catálogo de categorías y perfiles | Implementado en FE-03; PR en revisión |
| `GET /spaces` | `listOwnSpaces` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `POST /spaces` | `createOwnSpaceDraft` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `GET /spaces/{spaceId}` | `getOwnSpaceDraft` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `PUT /spaces/{spaceId}` | `updateOwnSpaceDraft` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `DELETE /spaces/{spaceId}` | `deleteOwnPublication` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `PUT /spaces/{spaceId}/publication` | `setOwnSpacePublicationState` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `PUT /spaces/{spaceId}/publication-content` | `updateOwnPublishedSpaceContent` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `GET /spaces/{spaceId}/gallery` | `listOwnSyntheticSpaceGallery` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `POST /spaces/{spaceId}/gallery` | `createSyntheticOwnSpaceGalleryPhoto` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `PUT /spaces/{spaceId}/cover` | `selectOwnSyntheticSpaceCover` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `GET /spaces/{spaceId}/gallery/{photoId}` | `getOwnSyntheticSpaceGalleryPhoto` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `DELETE /spaces/{spaceId}/gallery/{photoId}` | `removeOwnSyntheticSpaceGalleryPhoto` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `GET /spaces/{spaceId}/gallery/{photoId}/content` | `getOwnSyntheticSpaceGalleryPhotoContent` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `PUT /spaces/{spaceId}/availability` | `configureOwnDraftAvailability` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `GET /spaces/{spaceId}/availability` | `queryOwnDraftAvailability` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `GET /spaces/{spaceId}/tariff` | `getOwnSpaceTariff` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `PUT /spaces/{spaceId}/tariff` | `updateOwnSpaceTariff` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `GET /spaces/{spaceId}/tariffs` | `listOwnSpaceTariffVersions` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `POST /spaces/{spaceId}/price-simulations` | `createPrivateOwnDraftPriceSimulation` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `GET /spaces/{spaceId}/price-simulations/{simulationId}` | `getOwnPrivatePriceSimulation` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `GET /spaces/{spaceId}/availability/blocks` | `listOwnDraftManualBlocks` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `POST /spaces/{spaceId}/availability/blocks` | `createOwnDraftManualBlock` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `GET /spaces/{spaceId}/availability/timezone` | `getOwnDraftTimeZone` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `DELETE /spaces/{spaceId}/availability/blocks/{blockId}` | `deleteOwnDraftManualBlock` | Bearer; permisos en Go | FE-06 — Gestión propia de espacios | Pendiente |
| `GET /local/booking-trial/fixture` | `getAuthorizedLocalBookingFixture` | Bearer; permisos en Go | Compatibilidad — Sustituida en UI por catálogo/detalle; conservar API | Sin UI directa justificada |
| `GET /local/booking-trial/catalog` | `searchAuthorizedLocalSyntheticCatalog` | Bearer; permisos en Go | FE-03 — Exploración, detalle y horarios | Implementado en FE-03; PR en revisión |
| `GET /local/booking-trial/catalog/{spaceId}` | `getAuthorizedLocalSyntheticCatalogDetail` | Bearer; permisos en Go | FE-03 — Exploración, detalle y horarios | Implementado en FE-03; PR en revisión |
| `GET /local/booking-trial/catalog/{spaceId}/availability-options` | `getAuthorizedLocalFixtureAvailabilityOptions` | Bearer; permisos en Go | FE-03 — Exploración, detalle y horarios | Implementado en FE-03; PR en revisión |
| `GET /local/booking-trial/catalog/{spaceId}/weekly-hours` | `getOwnSyntheticHourlyWeeklyHours` | Bearer; permisos en Go | FE-06 — Horario semanal propio | Pendiente |
| `PUT /local/booking-trial/catalog/{spaceId}/weekly-hours` | `replaceOwnSyntheticHourlyWeeklyHours` | Bearer; permisos en Go | FE-06 — Horario semanal propio | Pendiente |
| `POST /local/booking-trial/quotes` | `quoteAuthorizedLocalBooking` | Bearer; permisos en Go | FE-03 — Cotización confirmada por Go | Implementado en FE-03; PR en revisión |
| `GET /local/booking-trial/reservations` | `listOwnLocalTrialReservations` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Pendiente |
| `POST /local/booking-trial/reservations` | `requestLocalTrialReservation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Pendiente |
| `GET /local/booking-trial/reservations/{reservationId}` | `getOwnLocalTrialReservation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Pendiente |
| `POST /local/booking-trial/reservations/{reservationId}/payment` | `simulateLocalTrialPayment` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Pendiente |
| `GET /local/booking-trial/reservations/{reservationId}/settlement` | `getOwnLocalSettlement` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Pendiente |
| `POST /local/booking-trial/reservations/{reservationId}/settlement` | `requestLocalFakeSettlement` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Pendiente |
| `POST /local/booking-trial/reservations/{reservationId}/settlement-reconcile` | `reconcileLocalFakeSettlement` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Pendiente |
| `GET /local/booking-trial/reservations/{reservationId}/receipt` | `getOwnLocalSettlementReceipt` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Pendiente |
| `GET /local/booking-trial/reservations/{reservationId}/guarantee` | `getOwnSyntheticReservationGuarantee` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Pendiente |
| `POST /local/booking-trial/reservations/{reservationId}/guarantee` | `executeLocalSyntheticGuaranteeOperation` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Pendiente |
| `POST /local/booking-trial/reservations/{reservationId}/guarantee-reconcile` | `reconcileLocalGuaranteeFakeResult` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Pendiente |
| `POST /local/booking-trial/reservations/{reservationId}/financial-decision` | `recordAdminLocalFinancialDecision` | Bearer; permisos en Go | FE-10 — Garantía, liquidación y comprobante de ensayo | Pendiente |
| `POST /local/booking-trial/payment-events` | `receiveAuthenticatedLocalPaymentEvent` | HMAC Backend | Técnica — Callback HMAC; no UI ni secreto en Angular | Sin UI directa justificada |
| `POST /local/booking-trial/reservations/{reservationId}/decision` | `decideLocalTrialReservation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Pendiente |
| `GET /local/booking-trial/reservations/{reservationId}/cancellation-preview` | `previewLocalTrialReservationCancellation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Pendiente |
| `POST /local/booking-trial/reservations/{reservationId}/cancel` | `cancelLocalTrialReservation` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Pendiente |
| `POST /local/booking-trial/reservations/{reservationId}/refund` | `simulateLocalTrialRefund` | Bearer; permisos en Go | FE-07 — Reserva, historial, pago y cancelación | Pendiente |
| `GET /local/booking-trial/reservations/{reservationId}/messages` | `listOwnLocalReservationMessages` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Pendiente |
| `POST /local/booking-trial/reservations/{reservationId}/messages` | `sendOwnLocalReservationMessage` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Pendiente |
| `POST /local/booking-trial/reservations/{reservationId}/messages/read` | `advanceOwnLocalReservationMessageReadCursor` | Bearer; permisos en Go | FE-09 — Conversación, lectura y reseñas | Pendiente |
| `GET /local/booking-trial/reservations/{reservation_id}/disputes` | `listOwnReservationTrialDisputes` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Pendiente |
| `POST /local/booking-trial/reservations/{reservation_id}/disputes` | `openOwnHostedTrialDispute` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Pendiente |
| `GET /local/booking-trial/disputes/{dispute_id}/history` | `getOwnTrialDisputeHistory` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Pendiente |
| `GET /admin/disputes` | `listOpenLocalTrialDisputes` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Pendiente |
| `POST /admin/disputes/{dispute_id}/close` | `closeLocalTrialDispute` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Pendiente |
| `POST /auth/logout` | `logout` | Bearer; permisos en Go | FE-02 — Identidad y seguridad | Implementado; aprobado y fusionado en PR #275 |
| `POST /local/booking-trial/reservations/{reservationId}/contract` | `createOrGetSyntheticReservationContract` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Pendiente |
| `GET /local/booking-trial/contracts/{contractId}` | `getOwnSyntheticContract` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Pendiente |
| `POST /local/booking-trial/contracts/{contractId}/sign` | `signSyntheticContractAsParticipant` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Pendiente |
| `POST /local/booking-trial/contracts/{contractId}/reject` | `rejectSyntheticContractVersion` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Pendiente |
| `GET /local/booking-trial/contracts/{contractId}/document` | `downloadSignedSyntheticContract` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Pendiente |
| `POST /local/booking-trial/reservations/{reservationID}/check-in` | `recordSyntheticCheckIn` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Pendiente |
| `POST /local/booking-trial/reservations/{reservationID}/check-out` | `recordSyntheticCheckOut` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Pendiente |
| `POST /local/booking-trial/reservations/{reservationID}/reception` | `recordSyntheticHostReception` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Pendiente |
| `GET /local/booking-trial/reservations/{reservationID}/operations` | `listSyntheticRentalOperations` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Pendiente |
| `GET /local/booking-trial/reservations/{reservationID}/evidence/{evidenceID}` | `downloadSyntheticRentalOperationEvidence` | Bearer; permisos en Go | FE-08 — Contrato y operación con evidencia | Pendiente |
| `POST /local/booking-trial/reservations/{reservationID}/damage-claim/defense/evidence` | `createSyntheticDamageClaimDefenseEvidence` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Pendiente |
| `GET /local/booking-trial/reservations/{reservationID}/damage-claim` | `getOwnSyntheticDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Pendiente |
| `POST /local/booking-trial/reservations/{reservationID}/damage-claim` | `openSyntheticHostDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Pendiente |
| `POST /local/booking-trial/reservations/{reservationID}/damage-claim/defense` | `submitSyntheticDamageClaimDefense` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia y evidencia | Pendiente |
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
| `GET /admin/local/damage-claims` | `listOpenLocalSyntheticDamageClaims` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Pendiente |
| `GET /admin/local/damage-claims/{claimID}` | `getAdminLocalSyntheticDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Pendiente |
| `GET /admin/local/damage-claims/{claimID}/evidence/{evidenceID}` | `getAdminLocalDamageClaimEvidence` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Pendiente |
| `POST /admin/local/damage-claims/{claimID}/resolution` | `resolveLocalSyntheticDamageClaim` | Bearer; permisos en Go | FE-10 — Reclamo/incidencia administrativa | Pendiente |

## Trazabilidad FE-03 — [PR #276](https://github.com/HernanEspinozaDev/espaciGo/pull/276) en revisión

| Operación | Pantalla / componente | Validación | Evidencia visual |
| --- | --- | --- | --- |
| `GET /local/booking-trial/catalog` | `/explorar`: búsqueda autenticada, categorías y perfiles, filtros tipados, paginación opaca, plano sintético | 35 pruebas Angular; cursor reiniciado al limpiar; respuesta A descartada tras logout/login B; `int64` CLP preservado como decimal exacto | [Explorar escritorio](evidencia/FE03_explorar_escritorio.png) · [móvil](evidencia/FE03_explorar_movil.png) |
| `GET /local/booking-trial/catalog/{spaceId}` | `/espacios/:id`: detalle seguro sin dirección, propietario ni galería privada | Recorrido de componente con sesión real y cuenta sintética; respuestas tardías condicionadas por espacio y sesión | [Detalle escritorio](evidencia/FE03_detalle_escritorio.png) · [móvil](evidencia/FE03_detalle_movil.png) |
| `GET /local/booking-trial/catalog/{spaceId}/availability-options` | Selector de fecha local, duración e intervalos UTC distintos para horas DST repetidas | `America/New_York` de prueba: las dos 01:30 muestran offsets diferentes; fecha/duración invalida selección y cotización | [Detalle escritorio](evidencia/FE03_detalle_escritorio.png) · [móvil](evidencia/FE03_detalle_movil.png) |
| `POST /local/booking-trial/quotes` | Cotización real de Go con snapshot CLP exacto y aviso de ensayo | `int64` > `Number.MAX_SAFE_INTEGER`; selección devuelta comparada con espacio e intervalo actual; cotización no crea reserva | [Detalle escritorio](evidencia/FE03_detalle_escritorio.png) |

La UI reutiliza `M04Local` y sus tipos generados para categorías/perfiles. Catálogo, disponibilidad y cotización se leen como texto: los literales de CLP se convierten en cadenas antes de parsear el JSON y se formatean con `BigInt`; filtros CLP viajan como cadenas de consulta. No se aplica `Number` a importes. El Bearer se adjunta en memoria por solicitud, no se registra y no pasa por URL. Go sigue siendo autoridad para roles, permisos, disponibilidad, zona IANA, horarios y snapshot de cotización. El mapa solo representa `x/y` normalizados y está rotulado como sintético.

**Validación:** regresión Angular, `api:check`, build/prerender y prueba M05/M06 Go contra PostgreSQL temporal en `tmpfs`; cuenta y datos sintéticos. Incluye errores 401/403/404/409/422, permiso de arrendatario, DST, cotización y aislamiento de sesión. Sin pruebas destructivas en el volumen persistente, sin proveedores reales y con el mock conservado.

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
