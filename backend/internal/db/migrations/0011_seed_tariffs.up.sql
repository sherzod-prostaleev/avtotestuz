-- Initial paid duration-pass tariffs. Idempotent: ON CONFLICT keeps later
-- admin edits from being clobbered when migrations re-run on redeploy.
INSERT INTO tariff (code, days, price_uzs, old_price_uzs, badge, sort_order, active) VALUES
  ('nexia',  7,  24900,  34900,  NULL,         1, true),
  ('gentra', 30, 59900,  99900,  'popular',    2, true),
  ('malibu', 75, 109900, 199900, 'best_value', 3, true)
ON CONFLICT (code) DO NOTHING;

INSERT INTO tariff_translation (tariff_id, locale, name, description)
SELECT t.id, v.locale, v.name, v.description
FROM tariff t
JOIN (VALUES
  ('nexia',  'uz-Latn'::locale_code, 'Nexia',  '1 haftalik to''liq kirish'),
  ('nexia',  'uz-Cyrl'::locale_code, 'Nexia',  '1 ҳафталик тўлиқ кириш'),
  ('nexia',  'ru'::locale_code,      'Nexia',  'Полный доступ на 1 неделю'),
  ('gentra', 'uz-Latn'::locale_code, 'Gentra', '1 oylik to''liq kirish'),
  ('gentra', 'uz-Cyrl'::locale_code, 'Gentra', '1 ойлик тўлиқ кириш'),
  ('gentra', 'ru'::locale_code,      'Gentra', 'Полный доступ на 1 месяц'),
  ('malibu', 'uz-Latn'::locale_code, 'Malibu', '2,5 oylik to''liq kirish'),
  ('malibu', 'uz-Cyrl'::locale_code, 'Malibu', '2,5 ойлик тўлиқ кириш'),
  ('malibu', 'ru'::locale_code,      'Malibu', 'Полный доступ на 2,5 месяца')
) AS v(code, locale, name, description) ON v.code = t.code
ON CONFLICT (tariff_id, locale) DO NOTHING;
