// Small static outlines and outside masks. Both geometry and framing use the
// same continuous world copy, including countries crossing the date line.
import { countryMask } from "../../../lib/country-mask.js";
import { countries } from "../../../lib/countries.js";

export function getStaticPaths() {
  return countries.filter(c => !c.parent && c.iso_numeric && countryMask(c.iso_numeric)).map(c => ({ params: { id: c.id }, props: { numericID: c.iso_numeric } }));
}

export function GET({ props }) {
  return new Response(JSON.stringify(countryMask(props.numericID)), { headers: { "Content-Type": "application/geo+json" } });
}
