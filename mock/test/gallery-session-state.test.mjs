import test from "node:test";
import assert from "node:assert/strict";
import { galleryContextMatches } from "../../internal/mockserver/static/gallery-session-state.js";

test("gallery drops delayed blob after logout and same-account relogin", () => {
  const request={token:"session-old",accountID:"host",generation:4,spaceID:"space-a",revision:8};
  assert.equal(galleryContextMatches(request,{token:"",accountID:"",generation:5,spaceID:"",revision:9}),false);
  assert.equal(galleryContextMatches(request,{token:"session-new",accountID:"host",generation:6,spaceID:"space-a",revision:10}),false);
});

test("gallery drops delayed list or image when selected space changes", () => {
  const request={token:"session",accountID:"host",generation:2,spaceID:"space-a",revision:3};
  assert.equal(galleryContextMatches(request,{token:"session",accountID:"host",generation:2,spaceID:"space-b",revision:4}),false);
  assert.equal(galleryContextMatches(request,{token:"session",accountID:"host",generation:2,spaceID:"space-a",revision:3}),true);
});
