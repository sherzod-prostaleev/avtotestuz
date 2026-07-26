package billing

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"avtotest.uz/backend/internal/db/sqlc"
)

type ManualTgPublicSettings struct {
	Configured      bool       `json:"configured"`
	HasAPIID        bool       `json:"has_api_id"`
	HasAPIHash      bool       `json:"has_api_hash"`
	HasSession      bool       `json:"has_session"`
	HumoBotUsername string     `json:"humo_bot_username"`
	LastTestOK      *bool      `json:"last_test_ok,omitempty"`
	LastTestAt      *time.Time `json:"last_test_at,omitempty"`
	LastTestDetail  string     `json:"last_test_detail,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

type ManualTgCredentialsPlain struct {
	APIID           int    `json:"api_id"`
	APIHash         string `json:"api_hash"`
	Session         string `json:"session"`
	HumoBotUsername string `json:"humo_bot_username"`
}

func (s Service) GetManualTgPublicSettings(ctx context.Context) (ManualTgPublicSettings, error) {
	row, err := s.Q.GetManualTgSettings(ctx)
	if err != nil {
		return ManualTgPublicSettings{}, err
	}
	out := ManualTgPublicSettings{
		HasAPIID:        row.ApiIDEnc.Valid && row.ApiIDEnc.String != "",
		HasAPIHash:      row.ApiHashEnc.Valid && row.ApiHashEnc.String != "",
		HasSession:      row.SessionEnc.Valid && row.SessionEnc.String != "",
		HumoBotUsername: row.HumoBotUsername,
	}
	out.Configured = out.HasAPIID && out.HasAPIHash && out.HasSession
	if row.LastTestOk.Valid {
		v := row.LastTestOk.Bool
		out.LastTestOK = &v
	}
	if row.LastTestAt.Valid {
		t := row.LastTestAt.Time
		out.LastTestAt = &t
	}
	if row.LastTestDetail.Valid {
		out.LastTestDetail = row.LastTestDetail.String
	}
	if row.UpdatedAt.Valid {
		t := row.UpdatedAt.Time
		out.UpdatedAt = &t
	}
	return out, nil
}

func (s Service) GetManualTgCredentialsPlain(ctx context.Context) (ManualTgCredentialsPlain, error) {
	kek := s.kek()
	if kek == nil {
		return ManualTgCredentialsPlain{}, fmt.Errorf("encryption key not configured")
	}
	row, err := s.Q.GetManualTgSettings(ctx)
	if err != nil {
		return ManualTgCredentialsPlain{}, err
	}
	if !row.ApiIDEnc.Valid || !row.ApiHashEnc.Valid || !row.SessionEnc.Valid {
		return ManualTgCredentialsPlain{}, fmt.Errorf("telegram credentials incomplete")
	}
	apiIDRaw, err := decryptSecret(kek, row.ApiIDEnc.String)
	if err != nil {
		return ManualTgCredentialsPlain{}, err
	}
	apiHash, err := decryptSecret(kek, row.ApiHashEnc.String)
	if err != nil {
		return ManualTgCredentialsPlain{}, err
	}
	session, err := decryptSecret(kek, row.SessionEnc.String)
	if err != nil {
		return ManualTgCredentialsPlain{}, err
	}
	apiID, err := strconv.Atoi(strings.TrimSpace(string(apiIDRaw)))
	if err != nil || apiID <= 0 {
		return ManualTgCredentialsPlain{}, fmt.Errorf("invalid api_id")
	}
	return ManualTgCredentialsPlain{
		APIID:           apiID,
		APIHash:         string(apiHash),
		Session:         string(session),
		HumoBotUsername: row.HumoBotUsername,
	}, nil
}

type ManualTgUpdate struct {
	APIID           string
	APIHash         string
	Session         string
	HumoBotUsername string
}

func (s Service) UpdateManualTgSettings(ctx context.Context, upd ManualTgUpdate) (ManualTgPublicSettings, error) {
	kek := s.kek()
	if kek == nil {
		return ManualTgPublicSettings{}, fmt.Errorf("encryption key not configured")
	}
	params := sqlc.UpsertManualTgSettingsParams{}
	if strings.TrimSpace(upd.APIID) != "" {
		enc, err := encryptSecret(kek, []byte(strings.TrimSpace(upd.APIID)))
		if err != nil {
			return ManualTgPublicSettings{}, err
		}
		params.SetApiID = true
		params.ApiIDEnc = pgtype.Text{String: enc, Valid: true}
	}
	if strings.TrimSpace(upd.APIHash) != "" {
		enc, err := encryptSecret(kek, []byte(strings.TrimSpace(upd.APIHash)))
		if err != nil {
			return ManualTgPublicSettings{}, err
		}
		params.SetApiHash = true
		params.ApiHashEnc = pgtype.Text{String: enc, Valid: true}
	}
	if strings.TrimSpace(upd.Session) != "" {
		enc, err := encryptSecret(kek, []byte(strings.TrimSpace(upd.Session)))
		if err != nil {
			return ManualTgPublicSettings{}, err
		}
		params.SetSession = true
		params.SessionEnc = pgtype.Text{String: enc, Valid: true}
	}
	if strings.TrimSpace(upd.HumoBotUsername) != "" {
		params.SetBotUsername = true
		params.HumoBotUsername = strings.TrimPrefix(strings.TrimSpace(upd.HumoBotUsername), "@")
	}
	if _, err := s.Q.UpsertManualTgSettings(ctx, params); err != nil {
		return ManualTgPublicSettings{}, err
	}
	return s.GetManualTgPublicSettings(ctx)
}

// RecordManualTgTestResult stores admin/watcher connectivity probe outcome.
func (s Service) RecordManualTgTestResult(ctx context.Context, ok bool, detail string) error {
	return s.Q.SetManualTgTestResult(ctx, sqlc.SetManualTgTestResultParams{
		LastTestOk:     pgtype.Bool{Bool: ok, Valid: true},
		LastTestDetail: pgtype.Text{String: detail, Valid: detail != ""},
	})
}

// TestManualTgSettingsDecrypt verifies credentials decrypt and look complete.
// Full MTProto probe is done by humo-watcher; this is a safe admin-panel check.
func (s Service) TestManualTgSettingsDecrypt(ctx context.Context) (ok bool, detail string, err error) {
	creds, err := s.GetManualTgCredentialsPlain(ctx)
	if err != nil {
		_ = s.RecordManualTgTestResult(ctx, false, err.Error())
		return false, err.Error(), nil
	}
	detail = fmt.Sprintf("credentials ok; bot=@%s; api_id=%d; session_len=%d",
		creds.HumoBotUsername, creds.APIID, len(creds.Session))
	_ = s.RecordManualTgTestResult(ctx, true, detail)
	return true, detail, nil
}
