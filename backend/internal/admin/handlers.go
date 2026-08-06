package admin

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"avtotest.uz/backend/internal/billing"
	"avtotest.uz/backend/internal/billing/recon"
	"avtotest.uz/backend/internal/db/sqlc"
	"avtotest.uz/backend/internal/httpx"
	"avtotest.uz/backend/internal/push"
)

// Handler mounts /admin/v1 routes.
type Handler struct {
	Svc             Service
	Pool            *pgxpool.Pool
	Redis           *redis.Client
	Secret          []byte
	Billing         billing.Service
	MetricsSnapshot MetricsSnapshot
	Push            *push.Service

	// StationBinaryPath is the Windows agent binary served by
	// GET /b2b/orgs/{id}/installer.exe. See config.Config.StationBinaryPath.
	StationBinaryPath string
	// PublicBaseURL is embedded in every downloaded installer as both the API
	// and frontend origin the station agent talks to.
	PublicBaseURL string
}

// Routes mounts public auth + protected admin routes under the given router
// (caller should mount at /admin/v1).
func (h *Handler) Routes(r chi.Router) {
	store := Store{Pool: h.Pool}
	r.Post("/auth/login", h.login)
	r.Post("/auth/totp/verify", h.verifyTOTPLogin)
	r.Post("/auth/refresh", h.refresh)
	r.Post("/auth/logout", h.logout)

	// TOTP enrollment is the one thing an admin locked out by
	// ADMIN_TOTP_ENFORCE must still be able to do, so it accepts the scoped
	// enrollment token as well as a full session. Disable stays behind
	// Required below: turning a second factor OFF is not part of getting in.
	r.Group(func(er chi.Router) {
		er.Use(TOTPEnrollAuth(h.Secret, store))
		er.Post("/security/totp/enroll", h.enrollTOTP)
		er.Post("/security/totp/confirm", h.confirmTOTP)
	})

	r.Group(func(pr chi.Router) {
		pr.Use(Required(h.Secret, store))
		pr.Get("/me", h.me)
		pr.Get("/ping", h.ping)
		pr.Post("/security/totp/disable", h.disableTOTP)

		pr.Group(func(ur chi.Router) {
			ur.Use(RequirePermission("users.read"))
			ur.Get("/users", h.listUsers)
			ur.Get("/users/{id}", h.getUser)
			ur.Get("/users/{id}/sessions", h.listUserSessions)
		})
		pr.Group(func(ur chi.Router) {
			ur.Use(RequirePermission("users.block"))
			ur.Post("/users/{id}/block", h.blockUser)
			ur.Post("/users/{id}/unblock", h.unblockUser)
		})
		pr.Group(func(ur chi.Router) {
			ur.Use(RequirePermission("users.write"))
			ur.Post("/users/{id}/bypass-variant-progress", h.setUserBypassVariantProgress)
		})
		pr.Group(func(ur chi.Router) {
			ur.Use(RequirePermission("users.hard_delete"))
			ur.Delete("/users/{id}", h.hardDeleteUser)
		})
		pr.Group(func(ur chi.Router) {
			ur.Use(RequirePermission("users.entitlements.grant"))
			ur.Post("/users/{id}/grant", h.grantUserVIP)
		})
		pr.Group(func(ur chi.Router) {
			ur.Use(RequirePermission("users.sessions.revoke"))
			ur.Post("/users/{id}/sessions/revoke-all", h.revokeAllUserSessions)
			ur.Post("/users/{id}/sessions/{sessionID}/revoke", h.revokeUserSession)
		})

		pr.Group(func(cr chi.Router) {
			cr.Use(RequirePermission("content.questions.read"))
			cr.Get("/content/questions", h.listQuestions)
			cr.Get("/content/questions/{id}", h.getQuestion)
			cr.Get("/content/questions/{id}/revisions", h.listQuestionRevisions)
			cr.Get("/content/explanations", h.listExplanations)
			cr.Get("/content/tickets", h.listTickets)
			cr.Get("/content/signs", h.listSigns)
		})
		pr.Group(func(cr chi.Router) {
			cr.Use(RequirePermission("content.questions.write"))
			cr.Patch("/content/questions/{id}", h.patchQuestion)
		})
		pr.Group(func(cr chi.Router) {
			cr.Use(RequirePermission("content.verify"))
			cr.Post("/content/questions/{id}/explanation/verify", h.verifyQuestionExplanation)
			cr.Post("/content/explanations/bulk-verify", h.bulkVerifyExplanations)
		})

		pr.Group(func(prr chi.Router) {
			prr.Use(RequirePermission("payments.read"))
			prr.Get("/payments/transactions", h.listPayments)
			prr.Get("/payments/transactions/{id}", h.getPayment)
			prr.Get("/payments/providers", h.listPaymentProviders)
			prr.Get("/payments/recon", h.runPaymentRecon)
			prr.Get("/payments/manual/queue", h.listManualPayQueue)
			prr.Get("/payments/manual/events", h.listUnmatchedManualEvents)
			prr.Get("/payments/manual/telegram", h.getManualTgSettings)
		})
		pr.Group(func(prr chi.Router) {
			prr.Use(RequirePermission("payments.keys.manage"))
			prr.Patch("/payments/providers/{provider}", h.patchPaymentProvider)
		})
		pr.Group(func(prr chi.Router) {
			prr.Use(RequirePermission("payments.void"))
			prr.Post("/payments/transactions/{id}/void", h.voidPayment)
		})
		pr.Group(func(prr chi.Router) {
			prr.Use(RequirePermission("payments.manual.manage"))
			prr.Get("/payments/manual/cards", h.listManualPayCards)
			prr.Get("/payments/manual/cards/{id}/pan", h.revealManualPayCard)
			prr.Post("/payments/manual/cards", h.createManualPayCard)
			prr.Patch("/payments/manual/cards/{id}", h.updateManualPayCard)
			prr.Delete("/payments/manual/cards/{id}", h.retireManualPayCard)
			prr.Post("/payments/manual/queue/{id}/confirm", h.confirmManualPay)
			prr.Post("/payments/manual/queue/{id}/reject", h.rejectManualPay)
			prr.Post("/payments/manual/events/{id}/ignore", h.ignoreManualEvent)
			prr.Put("/payments/manual/telegram", h.putManualTgSettings)
			prr.Post("/payments/manual/telegram/test", h.testManualTgSettings)
		})
		pr.Group(func(rr chi.Router) {
			rr.Use(RequirePermission("referral.read"))
			rr.Get("/users/{id}/referral", h.getUserReferral)
		})
		pr.Group(func(rr chi.Router) {
			rr.Use(RequirePermission("referral.payouts.manage"))
			rr.Get("/referral/payouts", h.listReferralPayouts)
			rr.Get("/referral/payouts/{id}/card", h.revealReferralPayoutCard)
			rr.Post("/referral/payouts/{id}/paid", h.markReferralPayoutPaid)
			rr.Post("/referral/payouts/{id}/reject", h.rejectReferralPayout)
		})
		pr.Group(func(rr chi.Router) {
			rr.Use(RequirePermission("referral.rates.manage"))
			rr.Patch("/users/{id}/referral/rate", h.patchUserReferralRate)
			rr.Put("/users/{id}/referral/balance", h.putUserReferralBalance)
		})
		pr.Group(func(rr chi.Router) {
			rr.Use(RequirePermission("referral.read"))
			rr.Get("/users/{id}/referral/ledger", h.getUserReferralLedger)
		})

		pr.Group(func(cr chi.Router) {
			cr.Use(RequirePermission("cms.read"))
			cr.Get("/cms/contacts", h.getCMSContacts)
			cr.Get("/cms/home", h.getCMSHome)
			cr.Get("/cms/legal", h.getCMSLegal)
		})
		pr.Group(func(cr chi.Router) {
			cr.Use(RequirePermission("cms.write"))
			cr.Put("/cms/contacts", h.putCMSContacts)
			cr.Put("/cms/home", h.putCMSHome)
			cr.Put("/cms/legal", h.putCMSLegal)
		})

		pr.Group(func(mr chi.Router) {
			mr.Use(RequirePermission("monitoring.read"))
			mr.Get("/monitoring/health", h.getMonitoringHealth)
			mr.Get("/monitoring/stream", h.streamMonitoring)
			mr.Get("/monitoring/metrics", h.getMonitoringMetrics)
			mr.Get("/monitoring/jobs", h.listMonitoringJobs)
			mr.Get("/monitoring/feed", h.getMonitoringFeed)
			mr.Get("/monitoring/alerts", h.getMonitoringAlerts)
		})

		pr.Group(func(ar chi.Router) {
			ar.Use(RequirePermission("analytics.read"))
			ar.Get("/analytics/overview", h.getAnalyticsOverview)
		})

		pr.Group(func(ir chi.Router) {
			ir.Use(RequirePermission("investors.read"))
			ir.Get("/investors/overview", h.getInvestorsOverview)
		})

		pr.Group(func(fr chi.Router) {
			fr.Use(RequirePermission("settings.flags"))
			fr.Get("/settings/flags", h.listFeatureFlags)
			fr.Patch("/settings/flags/{key}", h.patchFeatureFlag)
		})

		pr.Group(func(lr chi.Router) {
			lr.Use(RequirePermission("settings.config"))
			lr.Get("/settings/limits", h.listLimitConfigs)
			lr.Patch("/settings/limits/{key}", h.patchLimitConfig)
		})

		pr.Group(func(sr chi.Router) {
			sr.Use(RequirePermission("security.audit.read"))
			sr.Get("/security/audit", h.listAdminAudit)
		})

		pr.Group(func(sr chi.Router) {
			sr.Use(RequirePermission("security.rbac"))
			sr.Get("/security/rbac", h.getSecurityRBAC)
		})

		pr.Group(func(sr chi.Router) {
			sr.Use(RequirePermission("support.inbox"))
			sr.Get("/support/tickets", h.listSupportTickets)
			sr.Get("/support/tickets/{id}", h.getSupportTicket)
			sr.Patch("/support/tickets/{id}", h.patchSupportTicket)
		})

		pr.Group(func(sr chi.Router) {
			sr.Use(RequirePermission("support.broadcast"))
			sr.Get("/support/banner", h.getSupportBanner)
			sr.Put("/support/banner", h.putSupportBanner)
			sr.Post("/support/broadcasts/webpush", h.postWebPushBroadcast)
		})

		pr.Group(func(br chi.Router) {
			br.Use(RequirePermission("users.read"))
			br.Get("/b2b/orgs", h.listB2BOrgs)
			br.Get("/b2b/orgs/{id}", h.getB2BOrg)
			br.Get("/b2b/orgs/{id}/stats", h.getB2BOrgStats)
			br.Get("/b2b/orgs/{id}/stations", h.listB2BStations)
			br.Get("/b2b/orgs/{id}/partner-promos", h.listB2BPartnerPromos)
			br.Get("/b2b/orgs/{id}/installer", h.getB2BInstaller)
			br.Get("/b2b/orgs/{id}/installer.exe", h.downloadB2BInstaller)
		})
		pr.Group(func(br chi.Router) {
			br.Use(RequirePermission("users.entitlements.grant"))
			br.Post("/b2b/orgs", h.createB2BOrg)
			br.Patch("/b2b/orgs/{id}", h.patchB2BOrg)
			br.Post("/b2b/orgs/{id}/licenses", h.createB2BLicense)
			br.Post("/b2b/orgs/{id}/partner-promos", h.createB2BPartnerPromo)
			br.Delete("/b2b/orgs/{id}/stations/{stationID}", h.revokeB2BStation)
			br.Post("/b2b/orgs/{id}/installer", h.openB2BInstaller)
			br.Post("/b2b/orgs/{id}/installer/rotate", h.rotateB2BInstaller)
		})
		pr.Group(func(br chi.Router) {
			br.Use(RequirePermission("b2b.orgs.hard_delete"))
			br.Delete("/b2b/orgs/{id}", h.hardDeleteB2BOrg)
		})
	})
}

type loginBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	ip := clientIP(r)
	out, err := h.Svc.Login(r.Context(), body.Email, body.Password, r.UserAgent(), ip)
	if err != nil {
		switch err {
		case ErrInvalidCreds, ErrDisabled, ErrLoginThrottled:
			// Audit every rejected attempt (never the password), so a
			// guessing run is visible to security.audit.read instead of
			// leaving the log showing only the eventual success.
			_ = h.Svc.Store.WriteAudit(r.Context(), nil, "admin.login.failed", "admin_user", "", nil, map[string]any{
				"email":  body.Email,
				"reason": err.Error(),
			}, ip, r.UserAgent(), middleware.GetReqID(r.Context()))
		}
		switch err {
		case ErrInvalidCreds:
			httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		case ErrDisabled:
			httpx.Error(w, http.StatusForbidden, "disabled", "admin account disabled")
		case ErrLoginThrottled:
			httpx.Error(w, http.StatusTooManyRequests, "rate_limited", "too many login attempts, try again later")
		default:
			httpx.Error(w, http.StatusInternalServerError, "internal", "login failed")
		}
		return
	}
	if out.RequiresTOTPSetup {
		_ = h.Svc.Store.WriteAudit(r.Context(), &out.Me.ID, "admin.login.totp_setup_required", "admin_user",
			out.Me.ID.String(), nil, map[string]any{"email": out.Me.Email}, ip, r.UserAgent(),
			middleware.GetReqID(r.Context()))
		writeTOTPSetupRequired(w, out.EnrollToken)
		return
	}
	if out.RequiresTOTP {
		httpx.Data(w, http.StatusOK, map[string]any{
			"requires_totp":   true,
			"challenge_token": out.ChallengeToken,
		})
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &out.Me.ID, "admin.login", "admin_user", out.Me.ID.String(), nil, map[string]any{
		"email": out.Me.Email,
	}, ip, r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, map[string]any{
		"tokens": out.Pair,
		"admin":  out.Me,
	})
}

// writeTOTPSetupRequired answers a login that enforcement refused a session
// to: still 403 with the machine-readable code the admin UI keys on, but now
// carrying the scoped enrollment token, which is the only in-product way out
// of that state. httpx has no helper that writes data and error together, and
// a token has no business being smuggled inside an error message.
func writeTOTPSetupRequired(w http.ResponseWriter, enrollToken string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]any{
			"totp_setup_required": true,
			"enrollment_token":    enrollToken,
			"expires_in":          int(totpEnrollTTL.Seconds()),
			"enroll_url":          "/admin/v1/security/totp/enroll",
		},
		"error": httpx.ErrorBody{
			Code:    "totp_setup_required",
			Message: "two-factor authentication must be enrolled before signing in",
		},
	})
}

type totpVerifyBody struct {
	ChallengeToken string `json:"challenge_token"`
	Code           string `json:"code"`
}

func (h *Handler) verifyTOTPLogin(w http.ResponseWriter, r *http.Request) {
	var body totpVerifyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	ip := clientIP(r)
	pair, me, err := h.Svc.VerifyTOTPLogin(r.Context(), body.ChallengeToken, body.Code, r.UserAgent(), ip)
	if err != nil {
		switch err {
		case ErrTOTPInvalid, ErrInvalidCreds:
			httpx.Error(w, http.StatusUnauthorized, "invalid_totp", "invalid TOTP code")
		default:
			httpx.Error(w, http.StatusInternalServerError, "internal", "totp verify failed")
		}
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &me.ID, "admin.login", "admin_user", me.ID.String(), nil, map[string]any{
		"email": me.Email,
		"totp":  true,
	}, ip, r.UserAgent(), middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, map[string]any{
		"tokens": pair,
		"admin":  me,
	})
}

// totpCodeBody is the body of enroll-confirm and disable. Confirm takes no
// secret: the server re-derives the pending one, so a secret in the request is
// ignored (older admin UIs still send it — encoding/json drops it).
type totpCodeBody struct {
	Code string `json:"code"`
}

func (h *Handler) enrollTOTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	secret, url, err := h.Svc.BeginTOTPEnroll(r.Context(), claims.AdminUserID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "totp enroll failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"secret":      secret,
		"otpauth_url": url,
		"note":        "Confirm with POST /security/totp/confirm (code only) before the secret is stored. Valid for a short window; call this endpoint again if confirmation fails.",
	})
}

func (h *Handler) confirmTOTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	var body totpCodeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if err := h.Svc.ConfirmTOTPEnroll(r.Context(), claims.AdminUserID, body.Code); err != nil {
		if err == ErrTOTPInvalid {
			httpx.Error(w, http.StatusBadRequest, "invalid_totp", "invalid TOTP code")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "totp confirm failed")
		return
	}
	ip := clientIP(r)
	_ = h.Svc.Store.WriteAudit(r.Context(), &claims.AdminUserID, "admin.totp.enable", "admin_user",
		claims.AdminUserID.String(), nil, map[string]any{"enabled": true}, ip, r.UserAgent(),
		middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, map[string]any{"totp_enabled": true})
}

func (h *Handler) disableTOTP(w http.ResponseWriter, r *http.Request) {
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	var body totpCodeBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if err := h.Svc.DisableTOTP(r.Context(), claims.AdminUserID, body.Code); err != nil {
		if err == ErrTOTPInvalid {
			httpx.Error(w, http.StatusBadRequest, "invalid_totp", "invalid TOTP code")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "totp disable failed")
		return
	}
	ip := clientIP(r)
	_ = h.Svc.Store.WriteAudit(r.Context(), &claims.AdminUserID, "admin.totp.disable", "admin_user",
		claims.AdminUserID.String(), nil, map[string]any{"enabled": false}, ip, r.UserAgent(),
		middleware.GetReqID(r.Context()))
	httpx.Data(w, http.StatusOK, map[string]any{"totp_enabled": false})
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	pair, err := h.Svc.Refresh(r.Context(), body.RefreshToken, r.UserAgent(), clientIP(r))
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, "invalid_credentials", "invalid refresh token")
		return
	}
	httpx.Data(w, http.StatusOK, pair)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	_ = json.NewDecoder(r.Body).Decode(&body)
	_ = h.Svc.Logout(r.Context(), body.RefreshToken)
	httpx.Data(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	me, err := h.Svc.Me(r.Context(), claims.AdminUserID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "me failed")
		return
	}
	httpx.Data(w, http.StatusOK, me)
}

func (h *Handler) ping(w http.ResponseWriter, r *http.Request) {
	httpx.Data(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"permissions": PermsFromContext(r.Context()),
	})
}

type reasonBody struct {
	Reason string `json:"reason"`
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.Svc.Store.ListLearners(r.Context(), ListLearnersFilter{
		Q:      r.URL.Query().Get("q"),
		VIP:    r.URL.Query().Get("vip"),
		Status: r.URL.Query().Get("status"),
	}, page, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "users query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type grantVIPBody struct {
	Days int    `json:"days"`
	Note string `json:"note"`
}

func (h *Handler) grantUserVIP(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body grantVIPBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if body.Days < 1 || body.Days > 3660 {
		httpx.Error(w, http.StatusBadRequest, "invalid_days", "days must be between 1 and 3660")
		return
	}
	exists, err := h.Svc.Store.LearnerExists(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "user query failed")
		return
	}
	if !exists {
		httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	if h.Pool == nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "pool required")
		return
	}
	note := strings.TrimSpace(body.Note)
	if note == "" {
		note = "admin panel grant"
	}

	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "begin tx failed")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	svc := billing.Service{Q: sqlc.New(tx)}
	until, err := svc.GrantDays(ctx, id, body.Days, "admin", note, uuid.NullUUID{})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "grant failed")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "commit failed")
		return
	}

	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "users.entitlements.grant", "profile", id.String(),
		nil, map[string]any{
			"days":  body.Days,
			"until": until.UTC().Format(time.RFC3339),
			"note":  note,
		}, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	detail, err := h.Svc.Store.GetLearner(r.Context(), id)
	if err != nil {
		httpx.Data(w, http.StatusOK, map[string]any{
			"until": until.UTC().Format(time.RFC3339),
			"days":  body.Days,
		})
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"user":  detail,
		"until": until.UTC().Format(time.RFC3339),
		"days":  body.Days,
	})
}

func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	out, err := h.Svc.Store.GetLearner(r.Context(), id)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "user query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type hardDeleteBody struct {
	Confirm string `json:"confirm"`
}

func (h *Handler) hardDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body hardDeleteBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing claims")
		return
	}
	err := h.Svc.Store.HardDeleteLearner(r.Context(), id, body.Confirm, MutationAudit{
		AdminUserID: claims.AdminUserID,
		IP:          clientIP(r),
		UA:          r.UserAgent(),
		RequestID:   middleware.GetReqID(r.Context()),
	})
	if err != nil {
		switch {
		case IsNoRows(err):
			httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
		case errors.Is(err, ErrDeleteConfirmation):
			httpx.Error(w, http.StatusBadRequest, "confirmation_mismatch", "confirmation must match phone or DELETE")
		case errors.Is(err, ErrDeleteSelf):
			httpx.Error(w, http.StatusConflict, "self_delete_forbidden", "current admin cannot delete own profile")
		case errors.Is(err, ErrProtectedProfile):
			httpx.Error(w, http.StatusConflict, "protected_profile", "admin-shaped profiles cannot be deleted here")
		default:
			httpx.Error(w, http.StatusInternalServerError, "internal", "user hard delete failed")
		}
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"id": id, "deleted": true})
}

func (h *Handler) listUserSessions(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	exists, err := h.Svc.Store.LearnerExists(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "user query failed")
		return
	}
	if !exists {
		httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	out, err := h.Svc.Store.ListLearnerSessions(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "sessions query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) blockUser(w http.ResponseWriter, r *http.Request) {
	h.setUserBlocked(w, r, true)
}

func (h *Handler) unblockUser(w http.ResponseWriter, r *http.Request) {
	h.setUserBlocked(w, r, false)
}

type bypassVariantProgressBody struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) setUserBypassVariantProgress(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body bypassVariantProgressBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	before, after, err := h.Svc.Store.SetLearnerBypassVariantProgress(r.Context(), id, body.Enabled)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "bypass update failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "users.bypass_variant_progress", "profile", id.String(),
		map[string]any{"bypass_variant_progress": before},
		map[string]any{"bypass_variant_progress": after},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	detail, err := h.Svc.Store.GetLearner(r.Context(), id)
	if err != nil {
		httpx.Data(w, http.StatusOK, map[string]any{"bypass_variant_progress": after})
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{"user": detail})
}

func (h *Handler) setUserBlocked(w http.ResponseWriter, r *http.Request, block bool) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body reasonBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	reason := strings.TrimSpace(body.Reason)
	if reason == "" {
		httpx.Error(w, http.StatusBadRequest, "reason_required", "reason is required")
		return
	}
	target := "banned"
	action := "users.block"
	if !block {
		target = "active"
		action = "users.unblock"
	}
	before, after, err := h.Svc.Store.SetLearnerStatus(r.Context(), id, target)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "status update failed")
		return
	}
	revoked := int64(0)
	if block {
		revoked, _ = h.Svc.Store.RevokeAllLearnerSessions(r.Context(), id)
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, action, "profile", id.String(),
		map[string]any{"status": mapLearnerStatus(before)},
		map[string]any{"status": mapLearnerStatus(after), "reason": reason, "sessions_revoked": revoked},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	detail, err := h.Svc.Store.GetLearner(r.Context(), id)
	if err != nil {
		httpx.Data(w, http.StatusOK, map[string]any{
			"status":           mapLearnerStatus(after),
			"sessions_revoked": revoked,
		})
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"user":             detail,
		"sessions_revoked": revoked,
	})
}

func (h *Handler) revokeAllUserSessions(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	exists, err := h.Svc.Store.LearnerExists(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "user query failed")
		return
	}
	if !exists {
		httpx.Error(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	n, err := h.Svc.Store.RevokeAllLearnerSessions(r.Context(), id)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "revoke failed")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "users.sessions.revoke_all", "profile", id.String(),
		nil, map[string]any{"revoked": n}, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, map[string]any{"revoked": n})
}

func (h *Handler) revokeUserSession(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	sessionID, ok := parseUUIDParam(w, r, "sessionID")
	if !ok {
		return
	}
	okRevoke, err := h.Svc.Store.RevokeLearnerSession(r.Context(), id, sessionID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "revoke failed")
		return
	}
	if !okRevoke {
		httpx.Error(w, http.StatusNotFound, "not_found", "session not found or already revoked")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "users.sessions.revoke", "refresh_token", sessionID.String(),
		nil, map[string]any{"profile_id": id.String()}, clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) listQuestions(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.Svc.Store.ListQuestions(
		r.Context(),
		r.URL.Query().Get("q"),
		r.URL.Query().Get("category"),
		r.URL.Query().Get("validation"),
		r.URL.Query().Get("explanation"),
		page, limit,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "questions query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) listTickets(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.Svc.Store.ListTickets(r.Context(), r.URL.Query().Get("q"), page, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "tickets query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) listSigns(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	out, err := h.Svc.Store.ListSigns(
		r.Context(),
		r.URL.Query().Get("q"),
		r.URL.Query().Get("group"),
		page, limit,
	)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "signs query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getQuestion(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	out, err := h.Svc.Store.GetQuestion(r.Context(), id)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "question not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "question query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type patchQuestionBody struct {
	ValidationStatus string `json:"validation_status"`
}

func (h *Handler) patchQuestion(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body patchQuestionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if strings.TrimSpace(body.ValidationStatus) == "" {
		httpx.Error(w, http.StatusBadRequest, "validation_status_required", "validation_status is required")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	beforeDetail, err := h.Svc.Store.GetQuestion(r.Context(), id)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "question not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "question query failed")
		return
	}
	before, after, err := h.Svc.Store.SetQuestionValidationStatus(r.Context(), id, body.ValidationStatus)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "question not found")
			return
		}
		if err.Error() == "invalid status" {
			httpx.Error(w, http.StatusBadRequest, "invalid_status", "validation_status must be valid or quarantined")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "question update failed")
		return
	}
	_ = h.Svc.Store.InsertContentRevision(r.Context(), "question", id, &adminID,
		"validation_status:"+before+"→"+after, beforeDetail)
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "content.questions.patch", "question", id.String(),
		map[string]any{"validation_status": before},
		map[string]any{"validation_status": after},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	detail, err := h.Svc.Store.GetQuestion(r.Context(), id)
	if err != nil {
		httpx.Data(w, http.StatusOK, map[string]any{"validation_status": after})
		return
	}
	httpx.Data(w, http.StatusOK, detail)
}

func (h *Handler) listQuestionRevisions(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	if _, err := h.Svc.Store.GetQuestion(r.Context(), id); err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "question not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "question query failed")
		return
	}
	out, err := h.Svc.Store.ListContentRevisions(r.Context(), "question", id, 20)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "revisions query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) listExplanations(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	status := r.URL.Query().Get("status")
	out, err := h.Svc.Store.ListExplanations(r.Context(), status, page, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "explanations query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) verifyQuestionExplanation(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	if beforeDetail, err := h.Svc.Store.GetQuestion(r.Context(), id); err == nil {
		_ = h.Svc.Store.InsertContentRevision(r.Context(), "question", id, &adminID, "before_verify", beforeDetail)
	}
	before, after, err := h.Svc.Store.VerifyQuestionExplanation(r.Context(), id, adminID)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "explanation not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "verify failed")
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "content.verify", "explanation", id.String(),
		map[string]any{"status": before, "locale": "uz-Latn"},
		map[string]any{"status": after, "locale": "uz-Latn", "verified_by": adminID.String()},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	detail, err := h.Svc.Store.GetQuestion(r.Context(), id)
	if err != nil {
		httpx.Data(w, http.StatusOK, map[string]any{"status": after})
		return
	}
	httpx.Data(w, http.StatusOK, detail)
}

type bulkVerifyBody struct {
	QuestionIDs []string `json:"question_ids"`
}

func (h *Handler) bulkVerifyExplanations(w http.ResponseWriter, r *http.Request) {
	var body bulkVerifyBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if len(body.QuestionIDs) == 0 || len(body.QuestionIDs) > 100 {
		httpx.Error(w, http.StatusBadRequest, "invalid_ids", "question_ids must contain 1..100 ids")
		return
	}
	ids := make([]uuid.UUID, 0, len(body.QuestionIDs))
	for _, s := range body.QuestionIDs {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, "invalid_ids", "invalid question id")
			return
		}
		ids = append(ids, id)
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	okN, skipped, err := h.Svc.Store.BulkVerifyExplanations(r.Context(), ids, adminID)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "bulk verify failed")
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "content.verify.bulk", "explanation", "",
		nil, map[string]any{"verified": okN, "skipped": skipped, "requested": len(ids)},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, map[string]any{
		"verified":  okN,
		"skipped":   skipped,
		"requested": len(ids),
	})
}

func (h *Handler) listPayments(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	from, errFrom := parseOptionalTime(r.URL.Query().Get("from"))
	to, errTo := parseOptionalTime(r.URL.Query().Get("to"))
	if errFrom != nil || errTo != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_date", "from/to must be RFC3339 or YYYY-MM-DD")
		return
	}
	out, err := h.Svc.Store.ListPayments(r.Context(), PaymentListFilter{
		Status:   r.URL.Query().Get("status"),
		Provider: r.URL.Query().Get("provider"),
		From:     from,
		To:       to,
		Page:     page,
		Limit:    limit,
	})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "payments query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getPayment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	out, err := h.Svc.Store.GetPayment(r.Context(), id)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "payment not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "payment query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type voidPaymentBody struct {
	Reason string `json:"reason"`
}

func (h *Handler) voidPayment(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "id")
	if !ok {
		return
	}
	var body voidPaymentBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "JSON payload expected")
		return
	}
	claims, ok := FromContext(r.Context())
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing admin")
		return
	}
	err := h.Svc.Store.VoidPayment(
		r.Context(), id, claims.AdminUserID, body.Reason,
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "payment not found")
			return
		}
		if errors.Is(err, ErrPaymentCannotVoid) {
			httpx.Error(w, http.StatusConflict, "payment_settled", "settled or in-flight provider payment cannot be voided")
			return
		}
		if errors.Is(err, ErrPaymentVoidReason) {
			httpx.Error(w, http.StatusBadRequest, "invalid_reason", err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "void failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) listPaymentProviders(w http.ResponseWriter, r *http.Request) {
	out, err := h.Billing.ListProviderStatuses(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "providers query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) runPaymentRecon(w http.ResponseWriter, r *http.Request) {
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours <= 0 {
		hours = 24
	}
	if hours > 168 {
		hours = 168
	}
	to := time.Now().UTC()
	from := to.Add(-time.Duration(hours) * time.Hour)
	out, err := recon.Run(r.Context(), h.Pool, recon.Options{From: from, To: to})
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "recon failed")
		return
	}
	httpx.Data(w, http.StatusOK, map[string]any{
		"dry_run": true,
		"note":    "Local payment↔provider txn consistency only — no live Payme/Click API calls",
		"result":  out,
	})
}

type patchProviderBody struct {
	Enabled bool `json:"enabled"`
}

func (h *Handler) patchPaymentProvider(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	var body patchProviderBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	before, _ := h.Billing.ListProviderStatuses(r.Context())
	var beforeEnabled *bool
	for _, row := range before {
		if row.Provider == provider {
			v := row.Enabled
			beforeEnabled = &v
			break
		}
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	updatedBy := "admin:" + adminID.String()
	out, err := h.Billing.SetProviderEnabled(r.Context(), provider, body.Enabled, updatedBy)
	if err != nil {
		if strings.Contains(err.Error(), "unknown provider") {
			httpx.Error(w, http.StatusBadRequest, "invalid_provider", "provider must be payme, click, or manual")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "failed to update provider")
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "payments.providers.patch", "payment_provider", provider,
		map[string]any{"enabled": beforeEnabled},
		map[string]any{"enabled": out.Enabled},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getAnalyticsOverview(w http.ResponseWriter, r *http.Request) {
	out, err := h.Svc.Store.GetAnalyticsOverview(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "analytics query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getInvestorsOverview(w http.ResponseWriter, r *http.Request) {
	out, err := h.Svc.Store.GetAnalyticsOverview(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "investors overview query failed")
		return
	}
	// Same aggregates as analytics — gated by investors.read for investor_viewer role.
	out.Note = "Investor read-only stub: same honest SQL tiles as analytics.overview. Not Metabase/Grafana."
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) listFeatureFlags(w http.ResponseWriter, r *http.Request) {
	out, err := h.Svc.Store.ListFeatureFlags(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "flags query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type patchFlagBody struct {
	Value json.RawMessage `json:"value"`
}

func (h *Handler) patchFeatureFlag(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(chi.URLParam(r, "key"))
	if key == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_key", "key is required")
		return
	}
	var body patchFlagBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Value) == 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body or missing value")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	before, after, err := h.Svc.Store.SetFeatureFlagValue(r.Context(), key, body.Value, "admin:"+adminID.String())
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "flag not found")
			return
		}
		if strings.Contains(err.Error(), "boolean") ||
			strings.Contains(err.Error(), "percentage") ||
			strings.Contains(err.Error(), "allowlist") ||
			strings.Contains(err.Error(), "invalid value") {
			httpx.Error(w, http.StatusBadRequest, "invalid_value", err.Error())
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "flag update failed")
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "settings.flags.patch", "feature_flag", key,
		map[string]any{"value": json.RawMessage(before.Value), "type": before.Type},
		map[string]any{"value": json.RawMessage(after.Value), "type": after.Type},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, after)
}

func (h *Handler) listAdminAudit(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	action := r.URL.Query().Get("action")
	entityType := r.URL.Query().Get("entity_type")
	q := r.URL.Query().Get("q")
	out, err := h.Svc.Store.ListAdminAudit(r.Context(), action, entityType, q, page, limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "audit query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) getSecurityRBAC(w http.ResponseWriter, r *http.Request) {
	out, err := h.Svc.Store.GetRBACMatrix(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "rbac query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

func (h *Handler) listLimitConfigs(w http.ResponseWriter, r *http.Request) {
	out, err := h.Svc.Store.ListLimitConfigs(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, "internal", "limits query failed")
		return
	}
	httpx.Data(w, http.StatusOK, out)
}

type patchLimitBody struct {
	FreeValue *int32 `json:"free_value"`
	VipValue  *int32 `json:"vip_value"`
}

func (h *Handler) patchLimitConfig(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(chi.URLParam(r, "key"))
	if key == "" {
		httpx.Error(w, http.StatusBadRequest, "invalid_key", "key is required")
		return
	}
	var body patchLimitBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	if body.FreeValue == nil && body.VipValue == nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_body", "free_value or vip_value required")
		return
	}
	before, err := h.Svc.Store.GetLimitConfig(r.Context(), key)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "limit key not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "limit query failed")
		return
	}
	free := before.FreeValue
	vip := before.VipValue
	if body.FreeValue != nil {
		free = *body.FreeValue
	}
	if body.VipValue != nil {
		vip = *body.VipValue
	}
	if free < -1 || vip < -1 {
		httpx.Error(w, http.StatusBadRequest, "invalid_value", "values must be >= -1 (-1 = unlimited)")
		return
	}
	claims, _ := FromContext(r.Context())
	adminID := claims.AdminUserID
	_, after, err := h.Svc.Store.SetLimitConfigValues(r.Context(), key, free, vip, adminID)
	if err != nil {
		if IsNoRows(err) {
			httpx.Error(w, http.StatusNotFound, "not_found", "limit key not found")
			return
		}
		httpx.Error(w, http.StatusInternalServerError, "internal", "limit update failed")
		return
	}
	_ = h.Svc.Store.WriteAudit(r.Context(), &adminID, "settings.limits.patch", "limit_config", key,
		map[string]any{"free_value": before.FreeValue, "vip_value": before.VipValue},
		map[string]any{"free_value": after.FreeValue, "vip_value": after.VipValue},
		clientIP(r), r.UserAgent(), middleware.GetReqID(r.Context()),
	)
	httpx.Data(w, http.StatusOK, after)
}

func parseOptionalTime(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		u := t.UTC()
		return &u, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		u := t.UTC()
		return &u, nil
	}
	return nil, strconv.ErrSyntax
}

func parseUUIDParam(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	raw := chi.URLParam(r, name)
	id, err := uuid.Parse(raw)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "invalid_id", "invalid "+name)
		return uuid.Nil, false
	}
	return id, true
}

func clientIP(r *http.Request) *net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return nil
	}
	return &ip
}
