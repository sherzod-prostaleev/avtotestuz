-- Data-preserving indexes for foreign-key checks and UUID-pivot question draws.
-- Every index is additive: no row or business object is rewritten or removed.

CREATE INDEX IF NOT EXISTS admin_audit_log_admin_user_idx ON admin_audit_log (admin_user_id);
CREATE INDEX IF NOT EXISTS admin_role_permission_permission_idx ON admin_role_permission (permission_id);
CREATE INDEX IF NOT EXISTS admin_user_role_role_idx ON admin_user_role (role_id);
CREATE INDEX IF NOT EXISTS answer_image_idx ON answer (image_id);
CREATE INDEX IF NOT EXISTS arena_answer_answer_idx ON arena_answer (answer_id);
CREATE INDEX IF NOT EXISTS arena_answer_profile_idx ON arena_answer (profile_id);
CREATE INDEX IF NOT EXISTS arena_answer_question_idx ON arena_answer (question_id);
CREATE INDEX IF NOT EXISTS audit_log_actor_idx ON audit_log (actor_id);
CREATE INDEX IF NOT EXISTS b2b_invite_accepted_by_idx ON b2b_invite (accepted_by);
CREATE INDEX IF NOT EXISTS b2b_invite_created_by_idx ON b2b_invite (created_by);
CREATE INDEX IF NOT EXISTS category_mastery_category_idx ON category_mastery (category_id);
CREATE INDEX IF NOT EXISTS content_revision_editor_idx ON content_revision (editor_id);
CREATE INDEX IF NOT EXISTS entitlement_created_by_idx ON entitlement (created_by);
CREATE INDEX IF NOT EXISTS entitlement_payment_idx ON entitlement (payment_id);
CREATE INDEX IF NOT EXISTS exam_session_category_idx ON exam_session (category_id);
CREATE INDEX IF NOT EXISTS exam_session_sign_idx ON exam_session (sign_id);
CREATE INDEX IF NOT EXISTS exam_session_variant_idx ON exam_session (variant_id);
CREATE INDEX IF NOT EXISTS explanation_feedback_explanation_idx ON explanation_feedback (explanation_id);
CREATE INDEX IF NOT EXISTS manual_pay_event_matched_payment_idx ON manual_pay_event (matched_payment_id);
CREATE INDEX IF NOT EXISTS payment_promo_code_idx ON payment (promo_code_id);
CREATE INDEX IF NOT EXISTS payment_tariff_idx ON payment (tariff_id);
CREATE INDEX IF NOT EXISTS profile_referred_by_idx ON profile (referred_by);
CREATE INDEX IF NOT EXISTS promo_code_created_by_idx ON promo_code (created_by);
CREATE INDEX IF NOT EXISTS promo_redemption_payment_idx ON promo_redemption (payment_id);
CREATE INDEX IF NOT EXISTS promo_redemption_profile_idx ON promo_redemption (profile_id);
CREATE INDEX IF NOT EXISTS question_correct_answer_idx ON question (correct_answer_id);
CREATE INDEX IF NOT EXISTS question_image_idx ON question (image_id);
CREATE INDEX IF NOT EXISTS question_memory_question_idx ON question_memory (question_id);
CREATE INDEX IF NOT EXISTS referral_payout_processed_by_idx ON referral_payout (processed_by);
CREATE INDEX IF NOT EXISTS refresh_token_profile_idx ON refresh_token (profile_id);
CREATE INDEX IF NOT EXISTS saved_question_question_idx ON saved_question (question_id);
CREATE INDEX IF NOT EXISTS session_answer_answer_idx ON session_answer (answer_id);
CREATE INDEX IF NOT EXISTS session_answer_question_idx ON session_answer (question_id);
CREATE INDEX IF NOT EXISTS session_question_question_idx ON session_question (question_id);
CREATE INDEX IF NOT EXISTS sign_group_idx ON sign (group_id);
CREATE INDEX IF NOT EXISTS sign_image_idx ON sign (image_id);
CREATE INDEX IF NOT EXISTS telegram_quiz_session_question_idx ON telegram_quiz_session (question_id);
CREATE INDEX IF NOT EXISTS variant_progress_variant_idx ON variant_progress (variant_id);
CREATE INDEX IF NOT EXISTS variant_question_question_variant_idx ON variant_question (question_id, variant_id);

CREATE INDEX IF NOT EXISTS question_valid_id_idx
  ON question (id) WHERE validation_status = 'valid';
CREATE INDEX IF NOT EXISTS question_valid_category_id_idx
  ON question (category_id, id) WHERE validation_status = 'valid';
CREATE INDEX IF NOT EXISTS question_valid_image_presence_id_idx
  ON question ((image_id IS NOT NULL), id) WHERE validation_status = 'valid';
CREATE INDEX IF NOT EXISTS question_sign_sign_question_idx
  ON question_sign (sign_id, question_id);
