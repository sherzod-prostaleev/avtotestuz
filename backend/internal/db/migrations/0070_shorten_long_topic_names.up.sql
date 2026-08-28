-- 0070_shorten_long_topic_names.up.sql
--
-- Six topic names ran past the two lines a card gives them, so the end of
-- the name was simply invisible on the picker — worst on the two that ran
-- 111 and 182 characters. These keep the subject and drop the tail.
--
-- The three intersections_* are rewritten as a set rather than only the two
-- that overflowed: they sit next to each other in the grid, and a family
-- where one member is phrased differently reads as a mistake.

UPDATE category_translation SET name = 'Qatnov qismida joylashuv'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'lane_position');
UPDATE category_translation SET name = 'Қатнов қисмида жойлашув'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'lane_position');
UPDATE category_translation SET name = 'Расположение на проезжей части'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'lane_position');

UPDATE category_translation SET name = 'Tartibga solinmagan: asosiy yo‘l to‘g‘riga'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_straight');
UPDATE category_translation SET name = 'Тартибга солинмаган: асосий йўл тўғрига'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_straight');
UPDATE category_translation SET name = 'Нерегулируемые: главная дорога прямо'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_straight');

UPDATE category_translation SET name = 'Tartibga solinmagan: teng ahamiyatli'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_equal');
UPDATE category_translation SET name = 'Тартибга солинмаган: тенг аҳамиятли'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_equal');
UPDATE category_translation SET name = 'Нерегулируемые: равнозначные дороги'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_equal');

UPDATE category_translation SET name = 'Tartibga solinmagan: asosiy yo‘l o‘zgaradi'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_turns');
UPDATE category_translation SET name = 'Тартибга солинмаган: асосий йўл ўзгаради'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_turns');
UPDATE category_translation SET name = 'Нерегулируемые: главная дорога меняется'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_turns');

UPDATE category_translation SET name = 'Piyodalar o‘tish joyi va bekatlar'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'pedestrian_crossings_stops');
UPDATE category_translation SET name = 'Пиёдалар ўтиш жойи ва бекатлар'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'pedestrian_crossings_stops');
UPDATE category_translation SET name = 'Пешеходные переходы и остановки'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'pedestrian_crossings_stops');

UPDATE category_translation SET name = 'Velosiped, moped va aravalar'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'cyclists_mopeds_animals');
UPDATE category_translation SET name = 'Велосипед, мопед ва аравалар'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'cyclists_mopeds_animals');
UPDATE category_translation SET name = 'Велосипеды, мопеды и повозки'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'cyclists_mopeds_animals');

UPDATE category_translation SET name = 'Mansabdor shaxslar majburiyatlari'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'officials_duties');
UPDATE category_translation SET name = 'Мансабдор шахслар мажбуриятлари'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'officials_duties');
UPDATE category_translation SET name = 'Обязанности должностных лиц'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'officials_duties');
