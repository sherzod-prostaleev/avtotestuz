-- 0070_shorten_long_topic_names.down.sql
-- Restores the full names.

UPDATE category_translation SET name = 'Yo‘lning qatnov qismida transport vositalarining joylashuvi'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'lane_position');
UPDATE category_translation SET name = 'Йўлнинг қатнов қисмида транспорт воситаларининг жойлашуви'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'lane_position');
UPDATE category_translation SET name = 'Расположение транспортных средств на проезжей части'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'lane_position');

UPDATE category_translation SET name = 'Tartibga solinmagan chorrahalar asosiy yo‘l yo‘nalishi to‘g‘riga'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_straight');
UPDATE category_translation SET name = 'Тартибга солинмаган чорраҳалар асосий йўл йўналиши тўғрига'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_straight');
UPDATE category_translation SET name = 'Нерегулируемые перекрестки с главной дорогой в прямом направлении'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_straight');

UPDATE category_translation SET name = 'Tartibga solinmagan chorrahalar teng ahamiyatli'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_equal');
UPDATE category_translation SET name = 'Тартибга солинмаган чорраҳалар тенг аҳамиятли'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_equal');
UPDATE category_translation SET name = 'Нерегулируемые перекрестки равнозначных дорог'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_equal');

UPDATE category_translation SET name = 'Tartibga solinmagan chorrahalar asosiy yo‘l yo‘nalishi o‘zgarishi'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_turns');
UPDATE category_translation SET name = 'Тартибга солинмаган чорраҳалар асосий йўл йўналиши ўзгариши'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_turns');
UPDATE category_translation SET name = 'Нерегулируемые перекрестки с изменением направления главной дороги'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'intersections_main_turns');

UPDATE category_translation SET name = 'Piyodalarning o‘tish joylari va yo‘nalishli transport vositalarining bekatlari'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'pedestrian_crossings_stops');
UPDATE category_translation SET name = 'Пиёдаларнинг ўтиш жойлари ва йўналишли транспорт воситаларининг бекатлари'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'pedestrian_crossings_stops');
UPDATE category_translation SET name = 'Пешеходные переходы и остановки маршрутных транспортных средств'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'pedestrian_crossings_stops');

UPDATE category_translation SET name = 'Velosiped, moped va aravalar harakatlanishiga, shuningdek, hayvonlarni haydab o‘tishga doir qo‘shimcha talablar'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'cyclists_mopeds_animals');
UPDATE category_translation SET name = 'Велосипед, мопед ва аравалар ҳаракатланишига, шунингдек, ҳайвонларни ҳайдаб ўтишга доир қўшимча талаблар'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'cyclists_mopeds_animals');
UPDATE category_translation SET name = 'Дополнительные требования к движению велосипедов, мопедов и гужевых повозок, а также к перегону животных'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'cyclists_mopeds_animals');

UPDATE category_translation SET name = 'Mansabdor shaxslarning va fuqarolarning yo‘l harakati xavfsizligini taminlash, transport vositalarini yo‘lga chiqarish, raqam va taniqli belgilarini o‘rnatish bo‘yicha majburiyatlari'
 WHERE locale = 'uz-Latn'
   AND category_id = (SELECT id FROM category WHERE code = 'officials_duties');
UPDATE category_translation SET name = 'Мансабдор шахсларнинг ва фуқароларнинг йўл ҳаракати хавфсизлигини таъминлаш, транспорт воситаларини йўлга чиқариш, рақам ва таниқли белгиларини ўрнатиш бўйича мажбуриятлари'
 WHERE locale = 'uz-Cyrl'
   AND category_id = (SELECT id FROM category WHERE code = 'officials_duties');
UPDATE category_translation SET name = 'Обязанности должностных лиц и граждан по обеспечению безопасности дорожного движения, выпуску транспортных средств на линию, установке регистрационных и опознавательных знаков'
 WHERE locale = 'ru'
   AND category_id = (SELECT id FROM category WHERE code = 'officials_duties');
