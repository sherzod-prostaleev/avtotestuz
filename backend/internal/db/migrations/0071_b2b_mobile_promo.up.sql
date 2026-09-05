ALTER TABLE b2b_org
  ADD COLUMN mobile_promo_enabled boolean NOT NULL DEFAULT false,
  ADD COLUMN mobile_promo_url text NOT NULL DEFAULT '',
  ADD CONSTRAINT b2b_mobile_promo_url_length CHECK (octet_length(mobile_promo_url) <= 512),
  ADD CONSTRAINT b2b_mobile_promo_enabled_url CHECK (NOT mobile_promo_enabled OR mobile_promo_url <> '');
