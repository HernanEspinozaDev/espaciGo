-- Target-specific V80 -> V81 prerequisite verification; no migration or DDL.
-- Exact authorized target only. Read-only contract check; never execute V81 here.
BEGIN TRANSACTION READ ONLY;
SELECT pg_advisory_xact_lock(1163087943, 1296648018);
DO $target_guard$
BEGIN
  IF current_database() <> 'espacigo_wizard_20261106'
     OR session_user <> 'postgres' OR current_user <> 'postgres'
     OR current_setting('server_version_num') <> '180006' THEN
    RAISE EXCEPTION 'wrong V81 trial target or administrator identity';
  END IF;
END $target_guard$;
DO $ledger_guard$
DECLARE bad_count integer;
BEGIN
  WITH expected(version,name,checksum) AS (VALUES
        (1, 'V000001__m01_identity.sql', '34736e22df35ffbd88bb43a1071788ba88ac9e6b35f2a5aeba83b9b87bf161f8'),
        (2, 'V000002__m02_profile_rights.sql', 'e3f9c0b9e60244490ef40230b5af6a452deb63cd475b43b7378d9a3895877ec2'),
        (3, 'V000003__m03_verification.sql', '161a2fc06b46eb181b6ddec78eac4ff8e3020b48553429f435ce81c08ce4814f'),
        (4, 'V000004__m03_reason_codes.sql', 'f3942ba61513556253650fabc962843350e9c441fd384d8d5b8bd59ecf7362d1'),
        (5, 'V000005__m04_space_drafts.sql', 'f5ca7a06da755c8bbf8adba1ed7c539c69fb50743b3f58012079eb565b564f86'),
        (6, 'V000006__m04_extensible_category_attributes.sql', '67bf11f666e9c3f2687e7bd6c3c0fcd82f075c51ff012f874ed3638b64cf2968'),
        (7, 'V000007__m04_profile_attribute_order.sql', 'c39bc8c2c3a7161bec0dd0b01b1f7cb04ff87c70aae15ea6db46a6054b1780c5'),
        (8, 'V000008__m03_synthetic_evidence.sql', '896d0808f925f44f5bb82222b56fde369723b3579fae510c3bd3b1a84002113d'),
        (9, 'V000009__m06_draft_manual_occupancy.sql', 'e3fd2c98930f3d44280406a1698aa6d4594aa7b7e8ae708157c81ed4103ed900'),
        (10, 'V000010__m05_versioned_rates_and_private_simulations.sql', '291748cd18b0ff3424066c322e570a9ff206a386a92e08ad714a9d0709ef212b'),
        (11, 'V000011__m06_local_booking_trial.sql', '7d6e5dfda683164f434c70b415c7383a48d16120abc4d2d10ef0c64db710f452'),
        (12, 'V000012__m06_local_booking_conditions_snapshot.sql', 'daa604294fbab0c9d3ac027e3e2629a366c93c78795ea7f5edd75733dfd3a1cd'),
        (13, 'V000013__m06_booking_history_sequence.sql', '8aaa68c5b3f06a8b363bc8332bb7574e937417616610d5dc4df8ac26365f30de'),
        (14, 'V000014__m06_deterministic_history_backfill.sql', 'c5b25c4d450f7ff5f60ebac401e85506a8835ade8e80c3043d0ddd2706682ccb'),
        (15, 'V000015__m05_local_synthetic_catalog.sql', '55c2641621ca6dcba8c00108fb0d3bd607730a1544417f79388d2942c2a18525'),
        (16, 'V000016__m09_local_reservation_messages.sql', '21772b55240bd9e42dcee9a0c60267f87e0c5652e9d20a2d4d84526994cc4a06'),
        (17, 'V000017__m09_reservation_message_read_cursor.sql', 'd22702994f854621e3eca912ce07a320a3b83e7c9bf1ad2910f82d8ef3cfe127'),
        (18, 'V000018__m05_synthetic_fixture_locations.sql', '65ff0d76bafb9b7e78207aac40172cb45443291b2903c71165650174c9eac89b'),
        (19, 'V000019__m06_local_weekly_hours.sql', '99d516ecae702fd158204d985662d1cc0c70104a9cd520a926009860dc160d57'),
        (20, 'V000020__m06_local_flexible_cancellation.sql', 'a594aa744d8fa8ffcf1884561d4e0f0730ea1a35098f422b55b46b4e933570d0'),
        (21, 'V000021__m06_local_payment_inbox.sql', '3c3e72a6fcf9e1b6570362c7254a65cd6024c2838792898dfee5daeebd088a86'),
        (22, 'V000022__local_identity_history_and_outbox.sql', 'f2166790c2500956a2cbbf4bc16280a7bec11d3d36bf6096cc5ae666f9b5b4a8'),
        (23, 'V000023__local_privacy_suppression_review.sql', 'b38c23af4953e46c6d6c3b145d7c1c8212b0408ddc39e7eb434cd945ed355f2c'),
        (24, 'V000024__local_dispute_privacy_blocker.sql', 'ed27af7effe2c16f13896a4a6240cb62da46333158588755c4f835c6ded66e29'),
        (25, 'V000025__local_synthetic_privacy_suppression.sql', 'cf390257626036c35d4f0ca4715ca977a4ceab2b3032ced3e789f4a4098ded19'),
        (26, 'V000026__local_privacy_retention_purge_and_replay.sql', '38eaed77c3421a34c1c3361e33d281d5ea1418bfc1c0d0c2d57ac8e8c6846e7b'),
        (27, 'V000027__local_credential_notice_terminal_recovery.sql', 'a30512e5efcf43b47426ef2a771b8899104d65aad4851058e990915a81ef94ec'),
        (28, 'V000028__local_kyc_eligibility_and_history.sql', 'dd3ebc10d38f9000b4ec1183a40b8ed4222e14c2fc358b8cbe26f98b7896a1a7'),
        (29, 'V000029__local_privacy_kyc_eligibility.sql', '9ed0486313c8c6172cd7d133c08e39c6bed39fd3bbfb596b1a4a3bbbda1595b6'),
        (30, 'V000030__m02_synthetic_profile_photo_and_payout.sql', '1130f484103f2383cbe310f312823a1af571ffc6e73603e03acb8e564bef4220'),
        (31, 'V000031__m04_local_publication_lifecycle.sql', '7c18af2f8437bcbf7752dc021892c25d65033562f05eb40d0d400ad05e76483a'),
        (32, 'V000032__m04_synthetic_space_gallery.sql', '803d2026c722795b0c008cc9ab81ac9fa796642dc83505161d614d7df0b3f994'),
        (33, 'V000033__m04_gallery_candidate_cleanup.sql', 'f3b0d590ba5b92d4483c0e173a6de758f6f3b7d69bff3923950bca5bb9ee0425'),
        (34, 'V000034__local_synthetic_contract_signing.sql', '8c1e2bcc99df3f0f120e57ba087f627ce4c3148791d9626a1c4ba6f7d5dbbda4'),
        (35, 'V000035__local_synthetic_rental_operations.sql', '205e0f230d0bf69c15da08c48f3dc1abc0639bba4980372e9451a49658ce7c26'),
        (36, 'V000036__local_communication_reviews_and_notices.sql', '322fd98a8b921b905832bd75b4006d5fbed3f233334d400a6863c3a03aea2b09'),
        (37, 'V000037__local_communication_privacy_coordination.sql', 'ad45697b244bac64d5713145b89cadf1f1522fa723f99f63feda8aa497ffd665'),
        (38, 'V000038__local_damage_claim_resolution.sql', '88000b1e3f94e68efca57e36a38914483cbf0d10c62741c75f57b282a7291b25'),
        (39, 'V000039__local_damage_claim_privacy_retention.sql', '86bd6b0d684c33d4213eb47d68763f00edc8b5716e2fe5e85d46fbaf1916e083'),
        (40, 'V000040__local_guarantee_and_dispute_finance.sql', 'c749f178824ff5c1695483d2c32cde6b7c76794e247a5d423101ce7486058b0d'),
        (41, 'V000041__local_admin_account_block.sql', '4e31876d355c374c61f335f199441647ffc68fc7991bd3404596c7005cd6db58'),
        (42, 'V000042__admin_block_privacy_minimization.sql', '0bc20128faf1e745ad7d9d5e95b4b4596a06ccf2be00ebd12946bb274ba9a512'),
        (43, 'V000043__local_admin_financial_audit_actions.sql', 'cc4105234f80e6d27152ecf7ec35c50838789fc58c5b07d7e82209bb151eded4'),
        (44, 'V000044__local_fake_settlement_receipt.sql', 'f3ff70285a2c020b930eb34104be3c600cc0aeeb98f15cdd82f9b949a8addabf'),
        (45, 'V000045__local_settlement_privacy_obligations.sql', '8a7f12beb68ddaa7c5d063b2659b4c1a851fefffaac8f3db91e76f451bb54754'),
        (46, 'V000046__local_close_durable_notices.sql', '74e70bad1a0fb0d99dfb25a66e5dae169d957c62914e879fda18489501ab5736'),
        (47, 'V000047__local_privacy_retention_purgers.sql', 'cdde799f60c3648c26e1cc943a3ef53b0d09814e3b38e050720a87d6139dd8c9'),
        (48, 'V000048__local_defense_synthetic_evidence.sql', '4da2568b0aa12f228da51ebf3570a0e61f72a200d08a73823d578f80bb00ebee'),
        (49, 'V000049__local_m04_gallery_cover.sql', '2327da85443e0b4e182c02b94341e9288ee37a3e3ac8b3354bcfb4b12e423f64'),
        (50, 'V000050__local_publication_retirement.sql', '1ba7ea88a0cb0d8b663f7756655d91104c5f266c0aae5bf6e749bcacb8bcd02e'),
        (51, 'V000051__local_close_reservation_after_settlement.sql', 'ad9411b1477ee6512086147c3961dc950b4f0113c163556003c7c257d19bfce7'),
        (52, 'V000052__scoped_publication_rate_retirement.sql', '5c204afa60a4c83fd570b5c82f771711f1f890339bd42a23ad78190bbdee8330'),
        (53, 'V000053__local_close_contract_and_publication_retention.sql', '2c34092c9db8cab94c0e6ebab1e72d82b061eead465b913d46e91aeb77842e68'),
        (54, 'V000054__local_contract_signature_expiry_retention.sql', '7fc01ac4ee0ee83db339fadfe95fc032a25cd2621184abe4d2408245ce926af3'),
        (55, 'V000055__mpv2_02_public_search_and_locations.sql', 'a6b2e9b6eb055ea0375b35ebe881091e41ad1d0eacfa78d82896b9b8827ab6b6'),
        (56, 'V000056__mpv2_04_public_gallery.sql', '5cccdde76a613a82afbefb582d5ef2bab16512c9c7a0b0cfcdbf11a91710d5a0'),
        (57, 'V000057__mpv2_11_typed_public_category_filters.sql', '4da5888e15043f22576fd026934896aa6adebf3a49a65df43534c98a02d9be9d'),
        (58, 'V000058__gadm_01_global_administrator.sql', 'd902d5aab8a6ba0a18bdc4e332792e567c0e5a00a69c897c3274c93384418acc'),
        (59, 'V000059__gadm_03_audit_scope_and_query_indexes.sql', 'a097a82fd5ece1b66ca953e4d48073834adc2edf4d31596f7d57f6ae36dbe93a'),
        (60, 'V000060__gadm_local_bootstrap_restricted_role.sql', '75495ea50cdc2e6f5a61566dd725f5f623eca6133dcfdef55bde88498f424156'),
        (61, 'V000061__mpv2_12_hourly_publication_schedules.sql', 'cb3f719b3b846607abef9b8d356c980b47da1a22f5da38b55839112f2b619b95'),
        (62, 'V000062__gcp04_durable_privacy_replay_registry.sql', '6fc1ea07d47287fe4ddafaf6f613a8c2e88daf441b0d337d8b6b35eead1243e8'),
        (63, 'V000063__gcp04_privacy_suppression_claim_fencing.sql', 'a42c0943ff314c5199d6fdd87c13f6552eeeb69629e811f6d41853e4eb16a858'),
        (64, 'V000064__gcp04_privacy_cleanup_worker_role.sql', '9b791ae6950dd9f23d87c31ae59dcbeb1feb9d85631ac21fa95e1e9a6a4763bf'),
        (65, 'V000065__gcp06_bootstrap_audit_actor.sql', 'b1f7126f125bc9694e30d43205a509f34aa9e39ad8257456f666a431f5492fd8'),
        (66, 'V000066__gcp06_test_access_reset.sql', '05a751e9d9ab73bf2babadd0b3ad9ab170ee44ddf8ca7d441625d516f4cadcb6'),
        (67, 'V000067__inmuebles_residential_rentals.sql', '0d5052cee2e9c45911c00bd32ee97cc3eb9c656c0a680b998911fbbdb48e0bcb'),
        (68, 'V000068__img_01b_media_schema.sql', '5eeda41115017b5ba292db13215304f1e576002e015b6d46352ebd2de98f2626'),
        (69, 'V000069__img_01c_local_media_provider.sql', '9dc863b5e4b1edd7cd32bf1c2e48ecce6f9e36d6bc13072a3bf3e5cd8a433493'),
        (70, 'V000070__img_01c_durable_media_write_intents.sql', '71bcbcd91091013b91a89b3be9a152a46bd9f86f71565a898b50cb95701f6cc6'),
        (71, 'V000071__img_01c_terminal_media_write_intents.sql', '30d355260611615324ca70336ced97498340b0e2725b7a8828147af6adc82d14'),
        (72, 'V000072__img_01d_media_relation_trigger_scope.sql', '50b7f34d340c20a690c9506e68e9146a311363a526eb7449812d2674cf39f461'),
        (73, 'V000073__img_01e_privacy_media_cleanup.sql', 'ac2bfc0ba17e97ee40297dc75d8aaaa25f8506e5602a2d281d09415158670ada'),
        (74, 'V000074__first_business_action_roles.sql', 'a91df632b305468e1389ee72831e77121382e4c467d2cc279ea393e1bdc57bf8'),
        (75, 'V000075__auth_evaluation_consent_documents.sql', 'd6396472df02f2dd2cddd43994be064e6b0705c3fa2cb044bed970fcc9992220'),
        (76, 'V000076__spaces_gallery_capacity_replacement_v2.sql', 'c3ea6cbead8a41dfd283b5fe1393499228afb0e3447b4c2fccecc50ea39f7843'),
        (77, 'V000077__img01b_gallery_slot_record_guard.sql', '22b98284b303f7a9e69e88afc83c978a5bbab1840e1ed12097b9fe24f653218c'),
        (78, 'V000078__img01b_gallery_replay_expected_revision.sql', '6cb99e5b21db0d4c975d1419596b4850410fb47f7cb2d104481334a9eb082c9e'),
        (79, 'V000079__document_verification_cases.sql', '248ecdd6eac067a704871d24148d850b5a603a6577e59e89d9b03d9a74130e5b'),
        (80, 'V000080__document_verification_space_ownership_supersession.sql', 'fc2c3eacf10bc99d6ad4c66a7a5aa2c80a0592b45eeee6e938400c95f613c401')
  )
  SELECT count(*) INTO bad_count
  FROM expected e FULL JOIN public.schema_migrations l
    ON e.version=l.version AND e.name=l.name AND e.checksum=l.checksum
  WHERE e.version IS NULL OR l.version IS NULL;
  IF bad_count <> 0 OR (SELECT count(*) FROM public.schema_migrations) <> 80 THEN
    RAISE EXCEPTION 'expected exact V1..V80 source-matching ledger; mismatches %', bad_count;
  END IF;
END $ledger_guard$;
DO $v81_preconditions$
DECLARE
    migrator_oid oid;
    api_login_oid oid;
    schema_owner_oid oid;
    helper_oid oid;
    runtime_oid oid;
    private_schema_oid oid;
BEGIN
    IF current_database() <> 'espacigo_wizard_20261106' OR session_user <> 'postgres' OR current_user <> 'postgres' OR current_setting('server_version_num') <> '180006' THEN
        RAISE EXCEPTION 'wrong V81 trial target or administrator identity';
    END IF;
    SELECT oid INTO migrator_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_migrator_login'
      AND rolcanlogin AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb
      AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit;
    SELECT oid INTO api_login_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_api_login'
      AND rolcanlogin AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb
      AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit;
    SELECT oid INTO schema_owner_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner'
      AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb
      AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit;
    SELECT oid INTO helper_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_document_verification_fn'
      AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication
      AND NOT rolbypassrls AND NOT rolcanlogin AND NOT rolinherit;
    SELECT oid INTO runtime_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'
      AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb
      AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit;
    SELECT oid INTO private_schema_oid FROM pg_catalog.pg_namespace
     WHERE nspname='document_verification_private' AND nspowner=helper_oid;
    IF migrator_oid IS NULL OR api_login_oid IS NULL OR schema_owner_oid IS NULL
       OR helper_oid IS NULL OR runtime_oid IS NULL OR private_schema_oid IS NULL THEN
        RAISE EXCEPTION 'V81 prerequisite missing: provision restricted migration, API, runtime, and helper roles plus the owned private schema';
    END IF;
    IF NOT EXISTS (
           SELECT 1 FROM pg_catalog.pg_auth_members m
            WHERE m.roleid=schema_owner_oid AND m.member=migrator_oid
              AND m.set_option AND NOT m.inherit_option AND NOT m.admin_option
       )
       OR NOT EXISTS (
           SELECT 1 FROM pg_catalog.pg_auth_members m
            WHERE m.roleid=helper_oid AND m.member=schema_owner_oid
              AND m.set_option AND NOT m.inherit_option AND NOT m.admin_option
       )
       OR NOT EXISTS (
           SELECT 1 FROM pg_catalog.pg_auth_members m
            WHERE m.roleid=runtime_oid AND m.member=api_login_oid
              AND m.set_option AND NOT m.inherit_option AND NOT m.admin_option
       )
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=migrator_oid AND m.roleid<>schema_owner_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=api_login_oid AND m.roleid<>runtime_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=schema_owner_oid AND m.roleid<>helper_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.roleid=migrator_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.roleid=api_login_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.roleid=schema_owner_oid AND m.member<>migrator_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=helper_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.roleid=helper_oid AND m.member<>schema_owner_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=runtime_oid)
       OR NOT pg_catalog.has_schema_privilege(helper_oid, private_schema_oid, 'CREATE')
       OR NOT pg_catalog.has_schema_privilege(helper_oid, 'public', 'USAGE')
       OR EXISTS (
           SELECT 1 FROM (VALUES
               ('usuario','id','SELECT'),('usuario','estado','SELECT'),('usuario','id','UPDATE'),
               ('espacio','id','SELECT'),('espacio','propietario_id','SELECT'),('espacio','id','UPDATE')
           ) required(table_name,column_name,privilege)
            WHERE NOT EXISTS (
                SELECT 1 FROM pg_catalog.pg_class c
                JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
                JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
                CROSS JOIN LATERAL pg_catalog.aclexplode(a.attacl) acl
                 WHERE n.nspname='public' AND c.relname=required.table_name
                   AND a.attname=required.column_name AND acl.grantee=helper_oid
                   AND acl.grantor=c.relowner AND acl.privilege_type=required.privilege
                   AND NOT acl.is_grantable
            )
       )
       OR pg_catalog.has_schema_privilege(helper_oid, 'public', 'CREATE')
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_namespace n
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
              AND n.oid<>private_schema_oid
              AND pg_catalog.has_schema_privilege(helper_oid, n.oid, 'CREATE')
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_class c
            JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
            CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) acl
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
               AND c.relkind IN ('r','p','v','m','f') AND acl.grantee=helper_oid
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_class c
            JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
            JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
            CROSS JOIN LATERAL pg_catalog.aclexplode(a.attacl) acl
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema() AND c.relkind IN ('r','p','v','m','f')
              AND acl.grantee=helper_oid
              AND NOT (n.nspname='public' AND (
                    (c.relname='usuario' AND a.attname IN ('id','estado') AND acl.privilege_type='SELECT')
                    OR (c.relname='usuario' AND a.attname='id' AND acl.privilege_type='UPDATE')
                    OR (c.relname='espacio' AND a.attname IN ('id','propietario_id') AND acl.privilege_type='SELECT')
                    OR (c.relname='espacio' AND a.attname='id' AND acl.privilege_type='UPDATE')
              ) AND acl.grantor=c.relowner AND NOT acl.is_grantable)
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_class c
            JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
            CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) acl
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema() AND c.relkind='S'
              AND acl.grantee=helper_oid
       )
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_class c WHERE c.relowner=helper_oid AND c.relkind IN ('r','p','v','m','S','f'))
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc p WHERE p.proowner=helper_oid)
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_proc p
            CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl
            WHERE acl.grantee=helper_oid AND acl.privilege_type='EXECUTE'
       )
       OR pg_catalog.has_database_privilege(helper_oid,current_database(),'CREATE')
       OR pg_catalog.has_database_privilege(migrator_oid,current_database(),'CREATE')
       OR pg_catalog.has_database_privilege(api_login_oid,current_database(),'CREATE')
       OR pg_catalog.has_database_privilege(schema_owner_oid,current_database(),'CREATE')
       OR pg_catalog.has_database_privilege(runtime_oid,current_database(),'CREATE')
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_namespace n
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
              AND pg_catalog.has_schema_privilege(migrator_oid,n.oid,'CREATE')
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_namespace n
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
              AND pg_catalog.has_schema_privilege(schema_owner_oid,n.oid,'CREATE')
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_namespace n
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
              AND pg_catalog.has_schema_privilege(api_login_oid,n.oid,'CREATE')
       )
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_namespace n
            WHERE n.nspname NOT IN ('pg_catalog','information_schema')
              AND n.oid<>pg_catalog.pg_my_temp_schema()
              AND pg_catalog.has_schema_privilege(runtime_oid,n.oid,'CREATE')
       )
       OR NOT pg_catalog.has_table_privilege(schema_owner_oid,'public.schema_migrations','SELECT')
       OR NOT pg_catalog.has_table_privilege(schema_owner_oid,'public.schema_migrations','INSERT')
       OR EXISTS (
           SELECT 1
             FROM pg_catalog.pg_class c
             CROSS JOIN LATERAL pg_catalog.aclexplode(
                 COALESCE(c.relacl, pg_catalog.acldefault('r', c.relowner))
             ) acl
            WHERE c.oid='public.schema_migrations'::regclass
              AND (
                  (acl.grantee <> c.relowner AND acl.grantee <> schema_owner_oid)
                  OR (
                      acl.grantee = schema_owner_oid
                      AND (
                          acl.privilege_type NOT IN ('SELECT','INSERT')
                          OR acl.grantor <> c.relowner
                          OR acl.is_grantable
                      )
                  )
              )
       )
       OR EXISTS (
           SELECT 1
             FROM (VALUES ('SELECT'::text),('INSERT'::text)) expected(privilege_type)
            WHERE NOT EXISTS (
                SELECT 1
                  FROM pg_catalog.pg_class c
                  CROSS JOIN LATERAL pg_catalog.aclexplode(
                      COALESCE(c.relacl, pg_catalog.acldefault('r', c.relowner))
                  ) acl
                 WHERE c.oid='public.schema_migrations'::regclass
                   AND acl.grantee=schema_owner_oid
                   AND acl.grantor=c.relowner
                   AND acl.privilege_type=expected.privilege_type
                   AND NOT acl.is_grantable
            )
       )
       OR EXISTS (
           SELECT 1
             FROM pg_catalog.pg_attribute a
            WHERE a.attrelid='public.schema_migrations'::regclass
              AND a.attnum > 0 AND NOT a.attisdropped AND a.attacl IS NOT NULL
       )
       OR pg_catalog.pg_has_role(runtime_oid,helper_oid,'MEMBER') THEN
        RAISE EXCEPTION 'V81 prerequisite privilege contract is incomplete or too broad';
    END IF;
END;
$v81_preconditions$;
ROLLBACK;
