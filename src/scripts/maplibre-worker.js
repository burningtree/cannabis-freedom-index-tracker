// MapLibre GL JS 6 is ESM-only and loads its worker from a URL beside its own module. A bundler
// moves that module (and pre-bundles it in development), so the worker is imported here as an
// asset and its URL given to MapLibre before the first map is created.
import { setWorkerUrl } from "maplibre-gl";
import workerUrl from "maplibre-gl/dist/maplibre-gl-worker.mjs?url";

setWorkerUrl(workerUrl);
