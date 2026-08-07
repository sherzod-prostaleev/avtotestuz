-- Admin-only B2B: a school is a licence plus N stations. Nobody is attached to
-- it, so membership, invites and the never-used home-seat SKU all go.

DROP TABLE b2b_invite;
DROP TABLE b2b_org_member;

ALTER TABLE b2b_org_license DROP COLUMN home_seats;
