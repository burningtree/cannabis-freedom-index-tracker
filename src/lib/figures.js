// The figures a report gives, as short labels in reading order. Shared by the report pages (built
// from the files) and the draft preview (drawn in the browser from the AI's answer).
const fmtNum = (n) => n.toLocaleString("en-GB", { maximumFractionDigits: 1 });
const fmtMoney = (m) => `${m.currency} ${fmtNum(m.amount)}`;

// the figures a report gives, in reading order, as { key, label, value }
export function figureRows(f = {}) {
  const rows = [];
  if (f.arrested != null) rows.push({ key: "arrested", label: "Arrested", value: fmtNum(f.arrested) });
  if (f.charged != null) rows.push({ key: "charged", label: "Charged", value: fmtNum(f.charged) });
  if (f.plants_destroyed != null) rows.push({ key: "plants_destroyed", label: "Plants destroyed", value: fmtNum(f.plants_destroyed) });
  if (f.hectares != null) rows.push({ key: "hectares", label: "Hectares", value: fmtNum(f.hectares) });
  if (f.kg_seized != null) rows.push({ key: "kg_seized", label: "Cannabis", value: `${fmtNum(f.kg_seized)} kg` });
  if (f.prison) rows.push({ key: "prison", label: "Prison", value: f.prison });
  if (f.fine) rows.push({ key: "fine", label: "Fine", value: fmtMoney(f.fine) });
  if (f.deaths != null) rows.push({ key: "deaths", label: "Deaths", value: fmtNum(f.deaths) });
  if (f.value) rows.push({ key: "value", label: "Value (official estimate)", value: fmtMoney(f.value) });
  return rows;
}
