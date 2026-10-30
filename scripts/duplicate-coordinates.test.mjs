import test from "node:test";
import assert from "node:assert/strict";
import {
  haversineMeters,
  placeRelation,
  findDuplicateCoordinateGroups,
} from "./lib/duplicate-coordinates.mjs";

test("haversineMeters is near zero for identical points and scales for short gaps", () => {
  assert.equal(haversineMeters([0, 0], [0, 0]), 0);
  const about111m = haversineMeters([0, 0], [0.001, 0]);
  assert.ok(about111m > 100 && about111m < 120);
});

test("placeRelation distinguishes same labels, facilities, related tokens and unrelated localities", () => {
  assert.equal(placeRelation("NAIA Terminal 3, Pasay City", "NAIA Terminal 3, Pasay City"), "same_label");
  assert.equal(
    placeRelation("Sir Seewoosagur Ramgoolam International Airport", "Sir Seewoosagur Ramgoolam International Airport, Plaisance"),
    "nested_label"
  );
  assert.equal(
    placeRelation("NAIA Terminal 3, Pasay City", "Ninoy Aquino International Airport, Manila"),
    "shared_facility"
  );
  assert.equal(
    placeRelation("Mwabungo village, Ukunda, Kwale", "Romika, Mwabungo village, Ukunda, Kwale"),
    "nested_label"
  );
  assert.equal(
    placeRelation("KK Towers, Juja Town", "Juja Matatu Stage, Kiambu"),
    "overlapping_tokens"
  );
  assert.equal(
    placeRelation("Kumbalagodu, Bengaluru", "Mandwa village, Kotra tehsil, Udaipur"),
    "unrelated"
  );
});

test("findDuplicateCoordinateGroups flags unrelated pins and keeps shared facilities advisory", () => {
  const groups = findDuplicateCoordinateGroups(
    [
      { file: "a.md", country: "in", place: "Agartala city", precision: "locality", coordinates: [23.83, 91.28] },
      { file: "b.md", country: "in", place: "Sidhai Mohanpur village", precision: "locality", coordinates: [23.83, 91.28] },
      { file: "c.md", country: "mu", place: "Sir Seewoosagur Ramgoolam International Airport", precision: "exact", coordinates: [-20.43, 57.68] },
      { file: "d.md", country: "mu", place: "Sir Seewoosagur Ramgoolam International Airport, Plaisance", precision: "locality", coordinates: [-20.43, 57.68] },
      { file: "e.md", country: "ke", place: "Voi town", precision: "locality", coordinates: [-3.55, 38.8] },
    ],
    { meters: 150 }
  );
  assert.equal(groups.length, 2);
  const review = groups.find((g) => g.kind === "review");
  assert.ok(review);
  assert.deepEqual(review.places.sort(), ["Agartala city", "Sidhai Mohanpur village"]);
  const facility = groups.find((g) => g.kind === "shared_facility" || g.kind === "same_place" || g.kind === "related");
  assert.ok(facility);
  assert.equal(facility.country, "mu");
});

test("near-duplicate pins within the meter threshold cluster together", () => {
  const groups = findDuplicateCoordinateGroups(
    [
      { file: "a.md", country: "np", place: "Ramchowk pickup stop", precision: "locality", coordinates: [26.41, 87.333] },
      { file: "b.md", country: "np", place: "Naya Basti paddy field", precision: "locality", coordinates: [26.4096, 87.3327] },
    ],
    { meters: 150 }
  );
  assert.equal(groups.length, 1);
  assert.equal(groups[0].kind, "review");
  assert.equal(groups[0].exact, false);
  assert.ok(groups[0].meters < 150);
});
