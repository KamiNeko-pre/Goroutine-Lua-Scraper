import assert from "node:assert/strict";
import test from "node:test";
import {
  dataset,
  fieldProfile,
  filterRows,
  csv,
  differences,
} from "./workbench.js";
const rule = {
  id: "catalog",
  name: "Catalog",
  source: "Catalog",
  item_key: "id",
  min_items: 1,
  schedule: "24h",
  display: { title: "label", score: "price", score_label: "CNY" },
};
const snapshot = (id, items) => ({
  task_id: id,
  rule_id: rule.id,
  status: "succeeded",
  finished_at: `2026-09-05T0${id}:00:00Z`,
  payload: { items },
});
test("rule version changes start a new comparison baseline", () => {
  const old = snapshot(1, [{ id: 1, price: 0 }]),
    current = snapshot(2, [{ id: 1, price: 150 }]);
  old.payload.meta = { rule_version: "1" };
  current.payload.meta = { rule_version: "2" };
  const model = dataset([old, current], [rule]);
  assert.equal(model.sources[0].previous, null);
  assert.equal(model.rows[0].change, "baseline");
  assert.equal(model.rows[0].scoreDelta, null);
});
test("latest snapshot is not inflated by repeated observations or limited to forty rows", () => {
  const items = Array.from({ length: 70 }, (_, i) => ({
    id: i,
    label: `record ${i}`,
    price: i,
  }));
  const model = dataset([snapshot(1, items), snapshot(2, items)], [rule]);
  assert.equal(model.rows.length, 70);
  assert.equal(model.observations, 140);
  assert.equal(model.rows.filter((r) => r.change === "stable").length, 70);
  assert.equal(
    filterRows(model.rows, { page: 5, pageSize: 15 }).rows.length,
    10,
  );
});
test("score delta and missing metric retain zero instead of inventing growth", () => {
  const model = dataset(
    [
      snapshot(1, [{ id: 1, price: 0 }, { id: 2 }]),
      snapshot(2, [
        { id: 1, price: 5 },
        { id: 2, price: 6 },
      ]),
    ],
    [rule],
  );
  assert.equal(model.rows[0].scoreDelta, 5);
  assert.equal(model.rows[1].scoreDelta, null);
  assert.equal(
    differences({ extra: { price: 2 } }, { extra: { price: 3 } })[0].field,
    "extra.price",
  );
});
test("field profile distinguishes missing values, zero and categories", () => {
  const p = fieldProfile([{ n: 0 }, { n: 5 }, { n: 10 }, { n: null }], "n");
  assert.equal(p.median, 5);
  assert.equal(p.missing, 1);
  assert.equal(p.present, 3);
  assert.equal(
    fieldProfile([{ n: "a" }, { n: "b" }, { n: "a" }], "n").numeric,
    false,
  );
});
test("search, pagination and CSV retain every matching result and escape formulas", () => {
  const rows = dataset(
    [
      snapshot(1, [
        { id: 1, label: 'alpha, "beta"', price: 2 },
        { id: 2, label: "=1+1", price: 5 },
      ]),
    ],
    [rule],
  ).rows;
  assert.equal(filterRows(rows, { query: "alpha beta" }).total, 1);
  const output = csv(rows);
  assert.match(output, /"alpha, ""beta"""/);
  assert.match(output, /"'=1\+1"/);
});
test("bad payload and duplicate identity remain visible as quality problems", () => {
  const model = dataset(
    [snapshot(1, [{ id: "x" }, { id: "x" }, {}])],
    [rule],
    new Date("2026-09-05T01:01:00Z"),
  );
  assert.equal(model.sources[0].duplicates, 1);
  assert.equal(model.sources[0].missingKeys, 1);
  assert.equal(model.sources[0].health.status, "degraded");
});
