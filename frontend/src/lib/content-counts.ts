/**
 * Pre-fetch fallbacks only. The catalog is the source of truth: every place
 * that shows these numbers reads the live `GET /variants` list through
 * `useCatalogCounts()`, so a newly imported bilet or question shows up without
 * a code change. These constants are what the UI paints for the few hundred
 * milliseconds before that request resolves (and if it fails), so keep them
 * close to reality but never treat them as the count.
 */
export const OFFICIAL_TICKET_COUNT = 64;

/**
 * Official valid question-bank size (verified import). Cap custom practice
 * counts here — not at arbitrary UI ceilings like 200.
 */
export const OFFICIAL_QUESTION_COUNT = 1265;

/** Topics (categories) the bank is filed under. Pre-fetch fallback, as above. */
export const OFFICIAL_TOPIC_COUNT = 42;
