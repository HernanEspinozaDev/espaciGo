# White office provenance reconciliation — 2026-11-05

## Commons source

The parsed official Wikimedia Commons Action API response preserved in the adjacent JSON receipt resolves the requested page title to `File:White office space (Unsplash).jpg`, pageid `61681790`, origin URL `https://upload.wikimedia.org/wikipedia/commons/3/3c/White_office_space_%28Unsplash%29.jpg`, dimensions 5616×3744 and 2,146,421 bytes. Metadata identifies Artist `Breather` (`@breather`), `CC0`, and Creative Commons Zero Public Domain Dedication.

API request: https://commons.wikimedia.org/w/api.php?action=query&titles=File%3AWhite_office_space_%28Unsplash%29.jpg&prop=info%7Cimageinfo&iiprop=url%7Csize%7Csha1%7Cextmetadata&format=json&formatversion=2
No raw-response byte digest is asserted: the receipt preserves parsed API fields, not the original HTTP byte stream. Commons SHA-1 identifies the source JPG; the SHA-256 values below identify local JPGs and derivatives, not the API response.

## Local source integrity

Both authorized scratch copies (`authorized-gallery-corpus/white_office.jpg` and `authorized-gallery-corpus-after-fallback/white_office.jpg`) are 2,146,421 bytes and SHA-256 `7e8699acf8bf181856f41cdee23d1b7728cbc05d0d78ff323ca81d82b6538bfe`, matching the corpus manifest and Commons file size. Detailed paths and API fields are in the adjacent JSON receipt.

## BEFORE/AFTER reconciliation

The immutable BEFORE evidence reports white_office display 55,418 bytes and thumbnail 12,466 bytes. AFTER reports the same sizes, and its derivative hash table gives identical BEFORE/AFTER SHA-256 for both white_office outputs: display `7435ac8286e58c9007ad9c09d145a50757fbf3d853f86912a9878645903d1165`; thumbnail `f42eb5afd77964133006f0e26f552f85a8ba20cd78a094bab49af18958d73336`. Therefore fallback did not run for white_office; its row-level caveat has been corrected in AFTER to state that its output is unchanged. The historical BEFORE file was not edited. This establishes derivative identity, not a fresh remeasurement.

API response snapshot is preserved in JSON receipt fields; verify against the official Commons page/API if needed.

This dated receipt explicitly supersedes the old attribution-unverified/HTTP403 caveat in immutable BEFORE and the manifest's retained preliminary blockers. That warning records an earlier failed HTML retrieval, not current provenance. BEFORE remains byte-for-byte historical evidence. A fresh independent API request also matched Commons SHA-1 `644ff9db1197bb63fa6f60c5ac5a5ae28a62c9d7` against both local original JPGs.
