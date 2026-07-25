-- name: InsertGrandMockCertificate :one
INSERT INTO grand_mock_certificate (session_id, profile_id, share_code, score, total)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (session_id) DO UPDATE SET
  share_code = grand_mock_certificate.share_code
RETURNING *;

-- name: GetGrandMockCertificateBySession :one
SELECT * FROM grand_mock_certificate WHERE session_id = $1;

-- name: GetGrandMockCertificateByShareCode :one
SELECT * FROM grand_mock_certificate WHERE share_code = $1;
