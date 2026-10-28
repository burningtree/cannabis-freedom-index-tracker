// The kinds of enforcement a report can be about, in the order they are listed and filtered.
// Each has a colour for its dot on the map: the one place colour appears besides the photos.
export const TYPES = {
  eradication: { label: "Eradication", plural: "Eradications", color: "#5f9e57", help: "Fields or plants destroyed" },
  seizure: { label: "Seizure", plural: "Seizures", color: "#d9a43c", help: "Cannabis, money or property taken" },
  raid: { label: "Raid", plural: "Raids", color: "#d9733c", help: "Homes, shops, clubs or farms searched" },
  arrest: { label: "Arrest", plural: "Arrests", color: "#c94f4f", help: "People detained" },
  prosecution: { label: "Prosecution", plural: "Prosecutions", color: "#8f6fc9", help: "Charges and trials" },
  sentence: { label: "Sentence", plural: "Sentences", color: "#5c8fd6", help: "Prison terms, fines, caning" },
  execution: { label: "Execution", plural: "Executions", color: "#e8e6df", help: "People put to death for cannabis" },
  death: { label: "Death", plural: "Deaths", color: "#9a9890", help: "People killed in enforcement or in custody" },
};
export const TYPE_IDS = Object.keys(TYPES);

// How well a report's coordinates pin the place, from the spot itself to the whole country.
export const PRECISION = {
  exact: "the spot itself",
  locality: "the village, town or city",
  district: "the district or municipality",
  region: "the province, state or region",
  country: "the country",
};

// What has been checked: `verified` means the facts were read in at least one source that was
// opened; `reported` that they are taken from a report not yet checked; `disputed` that sources
// contradict each other or the authorities' account is contested.
export const STATUS = {
  verified: "Checked against the sources",
  reported: "As reported, not yet checked",
  disputed: "Sources disagree",
};
