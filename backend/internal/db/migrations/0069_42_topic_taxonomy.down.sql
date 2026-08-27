-- 0069_42_topic_taxonomy.down.sql
-- Restores the 13 legacy categories.

-- 1. Insert 13 old categories
INSERT INTO category (code, sort_order) VALUES
  ('road_signs_markings', 1),
  ('priority_intersections', 2),
  ('maneuvering_lane_position', 3),
  ('vehicle_equipment_lighting', 4),
  ('stopping_parking', 5),
  ('overtaking_speed', 6),
  ('pedestrians_public_transport', 7),
  ('special_road_zones', 8),
  ('traffic_signals_gestures', 9),
  ('towing_special_vehicles', 10),
  ('accidents_first_aid_dynamics', 11),
  ('cargo_passenger_carriage', 12),
  ('general_provisions_admin', 13)
ON CONFLICT (code) DO NOTHING;

-- 2. Insert 13 category translations
INSERT INTO category_translation (category_id, locale, name, status)
SELECT id, 'uz-Latn'::locale_code, 'Yo''l belgilari va chizig''i', 'verified' FROM category WHERE code = 'road_signs_markings'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Йўл белгилари ва чизиғи', 'verified' FROM category WHERE code = 'road_signs_markings'
UNION ALL
SELECT id, 'ru'::locale_code, 'Дорожные знаки и разметка', 'verified' FROM category WHERE code = 'road_signs_markings'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'Chorrahalar va yo''l ustunligi', 'verified' FROM category WHERE code = 'priority_intersections'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Чорраҳалар ва йўл устунлиги', 'verified' FROM category WHERE code = 'priority_intersections'
UNION ALL
SELECT id, 'ru'::locale_code, 'Перекрёстки и преимущество проезда', 'verified' FROM category WHERE code = 'priority_intersections'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'Manyovr va yo''lda joylashuv', 'verified' FROM category WHERE code = 'maneuvering_lane_position'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Манёвр ва йўлда жойлашув', 'verified' FROM category WHERE code = 'maneuvering_lane_position'
UNION ALL
SELECT id, 'ru'::locale_code, 'Манёврирование и расположение на проезжей части', 'verified' FROM category WHERE code = 'maneuvering_lane_position'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'Transport vositasi jihozi va yorug''lik', 'verified' FROM category WHERE code = 'vehicle_equipment_lighting'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Транспорт воситаси жиҳози ва ёруғлик', 'verified' FROM category WHERE code = 'vehicle_equipment_lighting'
UNION ALL
SELECT id, 'ru'::locale_code, 'Техническое состояние ТС и освещение', 'verified' FROM category WHERE code = 'vehicle_equipment_lighting'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'To''xtash va to''xtab turish', 'verified' FROM category WHERE code = 'stopping_parking'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Тўхташ ва тўхтаб туриш', 'verified' FROM category WHERE code = 'stopping_parking'
UNION ALL
SELECT id, 'ru'::locale_code, 'Остановка и стоянка', 'verified' FROM category WHERE code = 'stopping_parking'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'Quvib o''tish va tezlik', 'verified' FROM category WHERE code = 'overtaking_speed'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Қувиб ўтиш ва тезлик', 'verified' FROM category WHERE code = 'overtaking_speed'
UNION ALL
SELECT id, 'ru'::locale_code, 'Обгон и скорость движения', 'verified' FROM category WHERE code = 'overtaking_speed'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'Piyodalar, yo''lovchilar va yo''nalishli transport', 'verified' FROM category WHERE code = 'pedestrians_public_transport'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Пиёдалар, йўловчилар ва йўналишли транспорт', 'verified' FROM category WHERE code = 'pedestrians_public_transport'
UNION ALL
SELECT id, 'ru'::locale_code, 'Пешеходы, пассажиры и маршрутный транспорт', 'verified' FROM category WHERE code = 'pedestrians_public_transport'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'Maxsus yo''l uchastkalari', 'verified' FROM category WHERE code = 'special_road_zones'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Махсус йўл участкалари', 'verified' FROM category WHERE code = 'special_road_zones'
UNION ALL
SELECT id, 'ru'::locale_code, 'Особые участки дорог', 'verified' FROM category WHERE code = 'special_road_zones'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'Svetofor va tartibga soluvchi ishoralari', 'verified' FROM category WHERE code = 'traffic_signals_gestures'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Светофор ва тартибга солувчи ишоралари', 'verified' FROM category WHERE code = 'traffic_signals_gestures'
UNION ALL
SELECT id, 'ru'::locale_code, 'Сигналы светофора и регулировщика', 'verified' FROM category WHERE code = 'traffic_signals_gestures'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'Shatakka olish va maxsus transport', 'verified' FROM category WHERE code = 'towing_special_vehicles'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Шатакка олиш ва махсус транспорт', 'verified' FROM category WHERE code = 'towing_special_vehicles'
UNION ALL
SELECT id, 'ru'::locale_code, 'Буксировка и спецтранспорт', 'verified' FROM category WHERE code = 'towing_special_vehicles'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'YHH, tez tibbiy yordam va tormozlash', 'verified' FROM category WHERE code = 'accidents_first_aid_dynamics'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'ЙТҲ, тез тиббий ёрдам ва тормозлаш', 'verified' FROM category WHERE code = 'accidents_first_aid_dynamics'
UNION ALL
SELECT id, 'ru'::locale_code, 'ДТП, первая помощь и динамика торможения', 'verified' FROM category WHERE code = 'accidents_first_aid_dynamics'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'Yuk va odam tashish', 'verified' FROM category WHERE code = 'cargo_passenger_carriage'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Юк ва одам ташиш', 'verified' FROM category WHERE code = 'cargo_passenger_carriage'
UNION ALL
SELECT id, 'ru'::locale_code, 'Перевозка людей и грузов', 'verified' FROM category WHERE code = 'cargo_passenger_carriage'
UNION ALL
SELECT id, 'uz-Latn'::locale_code, 'Umumiy qoidalar va majburiyatlar', 'verified' FROM category WHERE code = 'general_provisions_admin'
UNION ALL
SELECT id, 'uz-Cyrl'::locale_code, 'Умумий қоидалар ва мажбуриятлар', 'verified' FROM category WHERE code = 'general_provisions_admin'
UNION ALL
SELECT id, 'ru'::locale_code, 'Общие положения и обязанности', 'verified' FROM category WHERE code = 'general_provisions_admin'
ON CONFLICT (category_id, locale) DO UPDATE SET name = EXCLUDED.name, status = 'verified';

-- 3. Re-point questions to default general_provisions_admin or previous category
UPDATE question SET category_id = (SELECT id FROM category WHERE code = 'general_provisions_admin' LIMIT 1);

UPDATE exam_session SET category_id = NULL WHERE category_id IN (SELECT id FROM category WHERE code IN ('general_rules', 'driver_duties', 'pedestrian_duties', 'special_vehicle_priority', 'signs_warning', 'signs_priority', 'signs_prohibitory', 'signs_mandatory', 'signs_information', 'signs_service', 'signs_additional', 'markings_horizontal', 'markings_vertical', 'traffic_lights', 'traffic_controller', 'warning_hazard_signals', 'starting_manoeuvring', 'lane_position', 'speed_limits', 'overtaking', 'stopping_and_parking', 'intersections_general', 'intersections_regulated', 'intersections_main_straight', 'intersections_equal', 'intersections_main_turns', 'pedestrian_crossings_stops', 'railway_crossings', 'motorways', 'residential_zones', 'slopes', 'public_transport_priority', 'lighting_devices', 'towing', 'driver_training', 'passenger_carriage', 'cargo_carriage', 'cyclists_mopeds_animals', 'officials_duties', 'vehicle_defects', 'safety_basics', 'first_aid'));

DELETE FROM category WHERE code IN ('general_rules', 'driver_duties', 'pedestrian_duties', 'special_vehicle_priority', 'signs_warning', 'signs_priority', 'signs_prohibitory', 'signs_mandatory', 'signs_information', 'signs_service', 'signs_additional', 'markings_horizontal', 'markings_vertical', 'traffic_lights', 'traffic_controller', 'warning_hazard_signals', 'starting_manoeuvring', 'lane_position', 'speed_limits', 'overtaking', 'stopping_and_parking', 'intersections_general', 'intersections_regulated', 'intersections_main_straight', 'intersections_equal', 'intersections_main_turns', 'pedestrian_crossings_stops', 'railway_crossings', 'motorways', 'residential_zones', 'slopes', 'public_transport_priority', 'lighting_devices', 'towing', 'driver_training', 'passenger_carriage', 'cargo_carriage', 'cyclists_mopeds_animals', 'officials_duties', 'vehicle_defects', 'safety_basics', 'first_aid');
