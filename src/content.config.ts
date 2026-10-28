// What a valid report looks like. The build fails with a clear message if a file in /reports breaks
// these rules: above all, a report without sources is refused.
import { defineCollection } from "astro:content";
import { glob } from "astro/loaders";
import { z } from "astro/zod";
import countries from "./data/countries.json";
import { TYPE_IDS, PRECISION, STATUS } from "./lib/types.js";

const COUNTRY_IDS = countries.map((c) => c.id) as [string, ...string[]];
const day = z.coerce.date();
const money = z.object({ amount: z.number().positive(), currency: z.string().regex(/^[A-Z]{3}$/) }).strict();

const source = z.object({
  title: z.string().min(1),
  url: z.string().url(),
  publisher: z.string().min(1),
  date: day.optional(), // when the source was published
  archived: z.string().url().optional(), // a copy in the Wayback Machine or archive.today
  language: z.string().regex(/^[a-z]{2}$/).optional(), // if not English
}).strict();

// What the enforcement did, in numbers, where a source gives them. Everything is optional;
// a number goes in only if a source states it.
const figures = z.object({
  arrested: z.number().int().nonnegative().optional(), // people detained
  charged: z.number().int().nonnegative().optional(), // people charged or tried
  kg_seized: z.number().nonnegative().optional(), // cannabis taken or destroyed, in kilograms
  plants_destroyed: z.number().int().nonnegative().optional(),
  hectares: z.number().nonnegative().optional(), // fields destroyed
  prison: z.string().min(1).optional(), // a sentence as stated: "5 months 24 days", "life"
  fine: money.optional(),
  value: money.optional(), // the authorities' estimate of what was seized or destroyed
  deaths: z.number().int().nonnegative().optional(),
}).strict();

const reports = defineCollection({
  // reports/<year>/<month>/<slug>.md; the slug alone is the report's id and address
  loader: glob({ pattern: "**/*.md", base: "./reports", generateId: ({ entry }) => entry.split("/").pop()!.replace(/\.md$/, "") }),
  schema: ({ image }) => z.object({
    date: day, // when it happened (or, for a campaign, when it ended or was announced)
    country: z.enum(COUNTRY_IDS), // a country file of the Cannabis Freedom Index: "de", "us/ca"
    place: z.string().min(1).optional(),
    coordinates: z.tuple([z.number().min(-90).max(90), z.number().min(-180).max(180)]).optional(), // [lat, lon]
    precision: z.enum(Object.keys(PRECISION) as [string, ...string[]]).optional(),
    type: z.enum(TYPE_IDS as [string, ...string[]]),
    also: z.array(z.enum(TYPE_IDS as [string, ...string[]])).optional(), // other kinds it involved
    title: z.string().min(1).max(120),
    summary: z.string().min(1).max(400),
    figures: figures.optional(),
    cover: z.object({
      image: image(),
      alt: z.string().min(1),
      credit: z.string().min(1),
      licence: z.string().min(1),
      url: z.string().url(), // where the picture and its licence can be checked
    }).strict().optional(),
    // photos or video of the event that may not be copied here (a newspaper's or an agency's):
    // shown as links to where they are published
    media: z.array(z.object({
      kind: z.enum(["photo", "video"]),
      url: z.string().url(),
      publisher: z.string().min(1),
      caption: z.string().min(1), // what it shows
      archived: z.string().url().optional(), // a copy in the Wayback Machine
    }).strict()).optional(),
    status: z.enum(Object.keys(STATUS) as [string, ...string[]]),
    tags: z.array(z.string().regex(/^[a-z0-9-]+$/)).optional(),
    sources: z.array(source).min(1, { message: "a report needs at least one source" }),
  }).strict()
    .refine((r) => !r.coordinates || r.precision, { message: "coordinates need a precision" })
    .refine((r) => !r.also?.includes(r.type), { message: "`also` repeats the main type" }),
});

export const collections = { reports };
