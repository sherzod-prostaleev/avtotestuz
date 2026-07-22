# Road signs catalog — sourcing research

Date: 2026-07-21
Scope: find a licensing-clean source + practical extraction path for a ~250-300 entry
Uzbekistan road-sign catalog (images + official codes + names in uz-Latn/uz-Cyrl/ru),
grouped by the 7 standard groups. Competitor sites explicitly excluded from any scraping.

All facts below are marked with the URL I actually fetched. Anything I could not verify
directly (page rendered but content was too large / paginated for the fetch tool to
surface fully) is flagged **UNVERIFIED / needs manual check**.

---

## 1. Options found

### Option A — `lex.uz`, Resolution of the Cabinet of Ministers No. 172 (12.04.2022),
"Йўл ҳаракати қоидаларини тасдиқлаш тўғрисида" (approving the Road Traffic Rules)

- Source: https://lex.uz/docs/5953883 (Uzbek Cyrillic), with sibling language versions
  linked from the same page: Uzbek Latin and Russian (Russian version at
  https://lex.uz/docs/5953887).
- Confirmed: the document's Annex 1 ("1-ИЛОВА / Йўл белгилари") is organized into the
  same 7 groups the product needs — Огоҳлантирувчи (warning), Имтиёз (priority),
  Тақиқловчи (prohibitory), Буюрувчи (mandatory), Ахборот-кўрсатгич (information),
  Сервис (service), Қўшимча ахборот (supplementary plates). Confirmed via table-of-
  contents anchors on the page.
- Confirmed: the page offers native "Ўзб / Рус / O'zb / Ўзб|Рус" language toggles, i.e.
  lex.uz genuinely publishes the same normative act in Uzbek Cyrillic, Uzbek Latin, and
  Russian — this is the single place official sign **names in all 3 target locales**
  can be sourced with confidence, straight from the primary legal text.
- **UNVERIFIED**: I could not get the fetch tool to surface the actual per-sign entries
  (image + code + name rows) inside Annex 1 — the page is very large and the tool kept
  returning table-of-contents/anchor structure rather than the leaf content. I could not
  confirm the exact `<img>` URL pattern lex.uz uses for the sign pictures, or their
  resolution/format. This needs a manual browser check or a scripted fetch of the raw
  HTML (not summarized) to find the image tags and confirm they're extractable
  (as opposed to being flattened into a scanned PDF image).
- Also: PDF/Word export buttons exist ("PDF форматида юклаб олиш", "MS Word'га сақлаш")
  — worth trying as an alternative extraction route if inline images turn out to be
  awkward to scrape from the rendered HTML.
- License basis: this is the literal text of a government resolution — see legal
  assessment below (Article 8 exclusion). This is the cleanest possible source from a
  licensing standpoint, if the extraction turns out to be practical.
- Effort: medium-high — need a person/script to actually open the page, locate Annex 1's
  DOM, and pull image URLs + adjacent code/name text for ~250-300 rows, across 3
  language variants. Not a bulk single-file download.

### Option B — `uzavtoyolbelgi.uz` (state enterprise "Uzavtoyo'lbelgi")

- Source: https://uzavtoyolbelgi.uz/uz/dorojnie/znaki
- Confirmed: this is the official state enterprise responsible for manufacturing/
  installing road signs in Uzbekistan (Tashkent address, banking details on the
  contact page — a davlat korxonasi, not a private business). The site hosts a catalog
  of **~300+ signs**, organized into the same 7 groups, each with a numeric code
  (e.g. "1.1", "2.4", "3.27"), an Uzbek name, and an image.
- Confirmed via `robots.txt` (https://uzavtoyolbelgi.uz/robots.txt): only
  `/yupe/backend*` and `/*/default*` are disallowed — the sign catalog pages themselves
  are not blocked, so a scripted crawl is not against the site's stated crawl policy.
- Caveat: footer shows "© Copyright 2026 «UZAVTOYO'LBELGI» | Barcha huquqlar
  himoyalangan" (all rights reserved) — see legal assessment below; this is a generic
  site-wide notice, and its force against the Article 8 statutory exclusion for
  normative-act content is genuinely unclear (uncertain — flagged below).
- This looks like the **best coverage** source (closest to the full 250-300 target,
  single site, consistent structure, codes clearly given) but the "all rights reserved"
  banner means it's the option needing the most legal comfort before bulk-downloading
  images from it, as opposed to treating the codes/names as normative fact (which are
  independently obtainable from lex.uz) and only using this site as a QA/cross-check
  and possibly a stopgap image source.
- Effort: medium — straightforward crawl (site is not blocking it), but needs the legal
  question above resolved first if we intend to reuse the actual image files.

### Option C — Wikimedia Commons (GOST-family SVG sets + Uzbekistan-specific redraws)

- `Category:Diagrams of Uzbek-language road signs` —
  https://commons.wikimedia.org/wiki/Category:Diagrams_of_Uzbek-language_road_signs —
  confirmed 41 files, named with codes (e.g. `UZ_road_sign_3.12.svg`,
  `5.41 Uzbekistan road sign.svg`). Checked one file's license page
  (https://commons.wikimedia.org/wiki/File:5.41_Uzbekistan_road_sign.svg): **CC BY-SA
  4.0**, "own work" by a Commons contributor (not an official scan) — meaning these are
  volunteer-redrawn vector recreations of the pictograms, not copies of a government
  document. CC BY-SA is commercially usable but requires (a) attribution and (b) that
  if you redistribute the SVG itself (modified or not), it stays under a compatible
  share-alike license. This does **not** require open-sourcing the app — only the
  image asset's own redistribution terms are affected.
- `Category:Diagrams of road signs of Uzbekistan` (the broader tree, 10 subcategories:
  route signs, additional, historic, information, priority, prohibitory, regulatory,
  service, warning, SVG-general) — confirmed to exist but I could only get exact file
  counts for two of the ten subcategories (SVG road signs: 2 files; priority: 3 files);
  the rest report only sub-subcategory counts, not leaf file counts. **Coverage is
  clearly partial** — nowhere near 250-300 signs live as dedicated Uzbekistan SVGs.
- Crucially: the **Uzbek Wikipedia article** "Oʻzbekiston yoʻl belgilari" —
  https://uz.wikipedia.org/wiki/O%CA%BBzbekiston_yo%CA%BBl_belgilari — confirmed to
  already assemble a near-complete catalog: ~180 signs across all 7 groups (35 warning,
  7 priority, 33 prohibitory, 9 mandatory, 47 information, 28 service, 21 supplementary),
  each row giving code + Uzbek Latin name + description, with images drawn from
  Wikimedia Commons. Critically, the image filenames mix `UZ_road_sign_X.X.svg`,
  `RU_road_sign_X.X.svg`, and `SU_road_sign_X.X.svg` — i.e. where no Uzbekistan-specific
  redraw exists, the article (and by extension, real practice) reuses the plain
  Russian-GOST pictogram, because Uzbekistan's current signs are visually
  near-identical to GOST R 52290-2004 signs except for the language of any text
  (confirmed independently by the English Wikipedia article, see below, which states
  Uzbek signs "follow Soviet GOST 10807-78... and Russian GOST R 52290-2004" with minor
  European-style design influences, and text rendered exclusively in Latin-script
  Uzbek).
- The **English Wikipedia article** "Road signs in Uzbekistan" —
  https://en.wikipedia.org/wiki/Road_signs_in_Uzbekistan — confirmed to show 200+ signs
  across the same 7 groups with codes 1.1–1.35, 2.1–2.7, 3.1–3.33, 4.1–4.9, 5.1–5.47,
  6.1–6.28, 7.1–7.23, plus a temporary (yellow background) signs section — all images
  sourced from Wikimedia Commons.
- The generic RU-GOST SVG set (`RU road sign X.X.svg`, referenced from a Commons user
  page "User:Nikolaev ec06ffa5/GOST-10807 based road signs") is a large, essentially
  complete pictogram library for the whole post-Soviet GOST family (confirmed files
  exist for RU, UA, BY sign codes at minimum). **UNVERIFIED**: I did not individually
  check the license tag on a sample of these RU-prefixed files — Commons file license
  templates for this whole family need a batch check before bulk use (I only verified
  the license on the one Uzbekistan-specific file above). Some simple GOST pictograms
  on Commons carry `{{PD-shape}}`/`{{PD-ineligible}}` tags (geometric shape, no
  originality), others are tagged CC-BY-SA "own work" by the uploader — this needs to be
  checked file-by-file or at least by uploader/batch before treating the whole set as
  one license.
- Effort: medium — Wikipedia articles (uz + en) are the fastest way to get a
  code-to-image-to-name mapping in bulk (structured, already grouped, already
  cross-referenced to Commons), but (a) coverage tops out around 180-200+ of the
  target 250-300, (b) image licenses are CC-BY-SA and need per-file/per-uploader
  attribution tracking, not a single blanket license, and (c) text-bearing signs
  (service/information signs with place names) will need Uzbek-Latin text substituted
  in wherever the reused image is the Russian original with Cyrillic place-name text —
  this is a real re-drawing task, not pure asset reuse.

### Option D — O'zDSt state standard for road signs

- Searched for the Uzbekistan-specific standard number (successor/localization of
  GOST 10807 / GOST R 52290 for use in Uzbekistan). Did not find a citable O'zDSt
  standard number or a digital copy of it (searches surfaced the general Uzbekistan
  Standardization Institute site, https://uzsti.uz/, and a standards catalog,
  https://standart.uz/page/view?id=47, but not the specific road-sign standard
  document itself). **UNVERIFIED / not resolved** — if this standard's designation
  and a digital copy of its pictogram plates can be found later, it would be a second
  strong "official normative document" source alongside the PDD Annex 1, potentially
  with the cleanest possible pictogram artwork (with dimensions/colors specified
  precisely). Not pursued further given time budget; flagged as an open question below.

---

## 2. Legal assessment — Uzbekistan copyright law and official documents

**Verified**: Law of the Republic of Uzbekistan No. LRU-42 of 20.07.2006 "On Copyright
and Related Rights" (as amended, e.g. by LRU-476/2018), **Article 8**, fetched from
https://lex.uz/docs/6123336 and cross-checked against the WIPO Lex English translation
(https://www.wipo.int/wipolex/edocs/lexdocs/laws/en/uz/uz004en.pdf, reached via redirect
from https://wipolex-res.wipo.int/edocs/lexdocs/laws/en/uz/uz004en.pdf).

Quoted original text (Uzbek), Article 8:

> "Қуйидагилар муаллифлик ҳуқуқи объектлари бўлмайди: расмий ҳужжатлар (қонунлар,
> қарорлар, тўхтамлар ва шу кабилар), шунингдек уларнинг расмий таржималари; расмий
> рамзлар ва белгилар (байроқлар, герблар, орденлар, пул белгилари ва шу кабилар); халқ
> ижодиёти асарлари; оддий матбуот ахбороти тусидаги кундалик янгиликларга доир ёки
> жорий воқеалар ҳақидаги хабарлар..."

In English: official documents (laws, resolutions, decisions, and the like) and their
official translations; official symbols and signs (flags, coats of arms, orders,
currency, and the like); works of folklore; and plain-press-style daily news reports
or reports of current events — **are not objects of copyright**.

This is also summarized (independently) on the Wikimedia Commons license-rules page for
Uzbekistan: https://commons.wikimedia.org/wiki/Commons:Copyright_rules_by_territory/Uzbekistan
— which states the same exclusions and cites the article as the basis for a
`{{PD-UZ-exempt}}` tag, and separately notes standard copyright term (life+70 years) and
that for Commons purposes a work must be PD in *both* Uzbekistan and the US (Uzbekistan
joined Berne 19.04.2005 and WIPO Copyright Treaty 17.07.2019).

**Application to this project**:
- The Road Traffic Rules themselves (Cabinet of Ministers Resolution No. 172, 2022,
  https://lex.uz/docs/5953883) are a government resolution — squarely "расмий ҳужжат"
  (official document) under Article 8. This means the **normative content** — the fact
  that sign 3.27 exists, its official code, its official name, its prescribed shape/
  colors/pictogram as specified in the annex — is not copyrightable in Uzbekistan.
  This is the strongest, most defensible basis for the whole project: rebuild the sign
  images from the specification (code + shape + pictogram description) rather than
  treat any single JPEG/PNG file as the "true" asset to copy byte-for-byte.
- This reasoning does **not** automatically extend to every third party's *particular
  digital rendering* of a sign. A photograph of a sign, or a specific SVG/PNG file
  someone drew, can carry its own copyright as a derivative artistic rendering — Article
  8 exempts the *official document*, not every subsequent depiction by every publisher.
  That's exactly why the Wikimedia Commons files are separately CC-BY-SA licensed by
  their uploaders (own work) rather than tagged PD — the uploaders' redrawing is treated
  as their own copyrightable expression of a public-domain fact pattern (the sign's
  meaning/shape), similar to how you can copyright your own rendering of a font showing
  public-domain numerals.
- The `uzavtoyolbelgi.uz` "all rights reserved" footer is **UNVERIFIED / uncertain** in
  its legal force: (a) if their sign images are literal reproductions of the annex to
  Resolution 172, the Article 8 exemption for the underlying official document likely
  still means the pictogram/shape/meaning can't be monopolized by the enterprise's
  copyright claim, but (b) if their site's specific photography/rendering/layout has
  independent creative elements (which "all rights reserved" suggests they believe),
  copying *their* image files verbatim is legally murkier than working from the lex.uz
  annex or GOST-standard shape directly. Practical recommendation: treat
  uzavtoyolbelgi.uz as a **reference/QA source for codes and current names**, not as
  the literal image asset source, unless legal counsel confirms otherwise.
- **Not resolved / uncertain**: whether the underlying pictogram *design* itself (as
  specified by whatever GOST/O'zDSt technical standard the PDD's annex incorporates by
  reference) is independently protectable as an industrial design or trademark by
  Uzavtoyo'lbelgi or a standards body, separate from ordinary copyright. This is a
  narrower and different IP question (industrial design / trademark, not copyright) that
  I did not investigate — flagged as an open question below. In practice, this risk is
  low for a driving-test-prep app (nominative/informational use of signs whose entire
  purpose is public recognition), but it's not something I can rule out with the same
  confidence as the copyright analysis.

---

## 3. Names/descriptions in 3 locales

- **uz-Latn and uz-Cyrl and ru**: confirmed available from lex.uz — the same
  resolution (No. 172, 2022) is published in Uzbek Cyrillic
  (https://lex.uz/docs/5953883), and linked from that page in Uzbek Latin and Russian
  (Russian version at https://lex.uz/docs/5953887). This is the one source that
  legitimately gives all 3 target locales from a single authoritative normative act,
  confirmed by the language-toggle UI present on the page ("Ўзб / Рус / O'zb /
  Ўзб|Рус").
- **uz-Latn** names are also cross-checkable in bulk against the Uzbek Wikipedia
  article (https://uz.wikipedia.org/wiki/O%CA%BBzbekiston_yo%CA%BBl_belgilari, ~180
  signs with Latin names) and uzavtoyolbelgi.uz's catalog (~300 signs, Uzbek names).
- **ru** names can be cross-checked against the English Wikipedia article's
  transliterations/descriptions and, if needed, the Russian Wikipedia equivalent
  (not directly checked in this pass — worth a follow-up look).
- **uz-Cyrl**: lex.uz's default rendering of Resolution 172 is already in Cyrillic, so
  this is directly available without needing algorithmic transliteration — though an
  automated Latin→Cyrillic transliterator could serve as a fallback/QA cross-check
  given Uzbek Latin↔Cyrillic is a fairly mechanical (if imperfect) transliteration.

---

## 4. Recommended path

**Primary**: Treat **lex.uz Resolution No. 172 (2022), Annex 1** as the source of legal
truth for codes, grouping, and names in all 3 locales (uz-Latn via the O'zb toggle,
uz-Cyrl via the default Ўзб view, ru via the Рус toggle /docs/5953887). Do not rely on
its embedded images alone until someone confirms they're crawlable (see open question
below) — if they are, harvest them directly since they carry the strongest possible
license story (official document, Article 8 exemption). If lex.uz's images turn out to
be low-res, embedded oddly, or otherwise impractical, fall back to redrawing pictograms
as clean in-house SVGs using the **code + shape/color/meaning specification** as the
source-of-truth "recipe," which sidesteps any derivative-work question entirely, styled
consistently across the whole catalog.

**Secondary / cross-check**: Use `uzavtoyolbelgi.uz`'s catalog (~300 signs, codes +
names, https://uzavtoyolbelgi.uz/uz/dorojnie/znaki) purely to validate completeness (make
sure no code from the 250-300 target set is missing) and as a visual reference when
redrawing pictograms in-house — not as a literal image-file copy source, given the
"all rights reserved" footer and unresolved question about their site's own creative
claim over their specific renderings.

**Tertiary / bootstrap**: Use the **Uzbek and English Wikipedia articles**
(uz.wikipedia.org/wiki/Oʻzbekiston_yoʻl_belgilari, en.wikipedia.org/wiki/Road_signs_in_Uzbekistan)
plus the Wikimedia Commons categories (`Category:Diagrams of Uzbek-language road signs`,
`Category:Diagrams of road signs of Uzbekistan`, and the broader GOST-family `RU road
sign X.X.svg` set) as a fast bootstrap for a first working version of the catalog:
already grouped, already coded, already has usable SVGs for a large fraction of signs.
Every image pulled from Commons must carry its own attribution (author + CC BY-SA 4.0 +
link) tracked per file — this is compatible with a commercial paid app (attribution +
share-alike on the asset, not on the app), but it is per-file bookkeeping, not a single
blanket license statement. Any sign with no dedicated Uzbek redraw available (i.e. where
Commons only offers the `RU_road_sign` GOST original) will need its embedded text
(if any) redone in Uzbek Latin — this applies mainly to information/service signs that
carry place names or text.

### Concrete next steps
1. Manually open https://lex.uz/docs/5953883 in a browser (not just the fetch-and-
   summarize tool) and inspect Annex 1's actual DOM/image tags to determine if a script
   can pull image URLs + adjacent code/name text. Try the PDF/Word export buttons on the
   page as an alternate extraction route if inline scraping is awkward.
2. Crawl `uzavtoyolbelgi.uz/uz/dorojnie/znaki` (robots.txt allows it) to build a
   code→name reference table (~300 rows) for completeness-checking against whatever
   catalog gets assembled from lex.uz + Commons.
3. Pull the Uzbek Wikipedia table (uz.wikipedia.org/wiki/Oʻzbekiston_yoʻl_belgilari) and
   English Wikipedia table (en.wikipedia.org/wiki/Road_signs_in_Uzbekistan) as CSV/JSON:
   code, group, name, Commons image filename. Cross-reference codes against the
   uzavtoyolbelgi.uz list from step 2 to find gaps (target ~250-300, Wikipedia only
   covers ~180-200+).
4. For each Commons image filename pulled in step 3, fetch its file page and record the
   actual license tag + author (do not assume all are CC BY-SA 4.0 "own work" — verify
   per file/per uploader batch, since the RU-prefixed GOST set was not individually
   checked in this pass).
5. For gaps (codes present in the official/enterprise catalog but missing a usable
   Commons SVG, and any sign where the only available image is the Russian-language
   GOST original with Cyrillic text), commission or draw in-house SVGs using the
   code + shape/meaning from lex.uz Annex 1 as the specification — this is the
   cleanest-licensed fallback since it's built from the exempted normative-document
   specification, not copied from any single third party's rendering.
6. Populate uz-Latn/uz-Cyrl/ru name fields from the three lex.uz language views
   directly; use Uzbek/English/Russian Wikipedia only as a cross-check, not as primary
   source of truth for names.

---

## 5. Risks and open questions

- **lex.uz image extractability is unverified.** The fetch tool could not surface actual
  `<img>` tags/URLs inside Annex 1 — this is the single biggest unknown blocking the
  "primary" path above. Needs a human or a raw-HTML/PDF-export check.
- **uzavtoyolbelgi.uz's own copyright claim over its specific images is unresolved.**
  Treat as reference-only until clarified; don't bulk-download and ship their exact
  image files without more certainty.
- **Commons license auditing is incomplete.** I verified only one file
  (`5.41 Uzbekistan road sign.svg`, CC BY-SA 4.0 own-work) as a sample. The larger
  `RU road sign X.X.svg` GOST-family set (potentially the biggest single pool of usable
  pictograms) needs a batch license check before bulk use — some files in that family
  may be tagged PD-ineligible/PD-shape (simple geometric shapes, no originality) rather
  than CC-BY-SA, which would be even better for the project, but this must be confirmed
  file-by-file or by uploader, not assumed.
- **Industrial-design/trademark protection of the pictogram design itself** (separate
  from copyright) by a standards body or Uzavtoyo'lbelgi was not investigated — flagged
  as a genuinely open legal question, though practically low-risk for a
  driving-test-prep app given the signs' whole purpose is public recognition.
  Recommend a brief sanity-check with counsel before large-scale commercial launch,
  even though the copyright analysis above is solid.
- **Coverage gap**: no single source confirmed to have all ~250-300 signs in one place
  with images. lex.uz (primary) should have full legal coverage of names/codes; images
  will likely need to be assembled from multiple sources (Commons + in-house redraws)
  to close the gap between the ~180-200 signs with ready Commons SVGs and the full
  target set.
- **O'zDSt standard number** for Uzbekistan's specific road-sign technical standard
  (successor to/localization of GOST 10807 / GOST R 52290) was not found in this pass —
  worth a follow-up search if a "cleanest possible" pictogram specification is wanted
  beyond the PDD annex text.
- **Text-bearing signs** (place names on information/service/direction signs) will need
  Uzbek Latin text drawn in wherever the only available reference image has Cyrillic
  (Russian) text — this is real design/production work, not just licensing/sourcing,
  and should be budgeted for separately from the "find a source" task.
