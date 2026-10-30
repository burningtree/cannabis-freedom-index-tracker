// Reviews are tied to the geographic fields, so edits cannot silently inherit
// an old exception. The raw geometric result remains visible in audit output.
export function matchingReview(reviews, file, data) {
  return reviews.find(review => review.file === file.replaceAll("\\", "/").replace(/^\.\//, "")
    && ["country", "place", "precision", "coordinates"].every(key => JSON.stringify(review[key]) === JSON.stringify(data[key]))
    && review.reason && review.evidence?.length > 0) ?? null;
}
