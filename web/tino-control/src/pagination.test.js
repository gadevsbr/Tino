import test from "node:test";
import assert from "node:assert/strict";
import { pageSlice } from "./pagination.js";

test("pagina falhas em blocos de vinte e corrige página fora do limite", () => {
  const items = Array.from({ length: 45 }, (_, id) => id + 1);
  assert.deepEqual(pageSlice(items, 2), { page: 2, pages: 3, items: items.slice(20, 40) });
  assert.deepEqual(pageSlice(items, 9), { page: 3, pages: 3, items: items.slice(40) });
  assert.deepEqual(pageSlice([], 1), { page: 1, pages: 1, items: [] });
});
