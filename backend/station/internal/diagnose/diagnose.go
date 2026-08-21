// Package diagnose turns an error from the backend or the network into
// something the person standing at a classroom PC can act on.
//
// It exists because the agent used to hand its operators raw wire codes. A
// school that registered a PC already belonging to another school saw exactly
// this, four times, and then a console that hung for two minutes:
//
//	enrollment failed: /api/v1/b2b/stations/enroll: conflict (conflict)
//
// Nothing in that line says which school, what to change, or who can change
// it. Worse, the agent retried it as if it were a dropped packet, because the
// retry loop had no notion of an error that will never succeed.
//
// So every failure gets three things here: whether retrying can possibly help
// (Retryable), what happened (Problem), and what to do about it (Action) --
// the last two in Uzbek, because that is the language of the room.
package diagnose

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"

	"avtotest.uz/station/internal/agent"
	"avtotest.uz/station/internal/status"
)

// Result is a classified failure.
type Result struct {
	// Phase is status.PhaseWaiting when the agent's own retries will fix
	// this, status.PhaseBlocked when only a human can.
	Phase status.Phase
	// Retryable reports whether trying the same call again could ever
	// succeed. Enrollment stops immediately when this is false.
	Retryable bool
	// Code is the backend's error code, or a synthetic one for local
	// failures ("network", "tls", "clock").
	Code string
	// Problem and Action are Uzbek, written for a school administrator.
	Problem string
	Action  string
	// Detail is the underlying error text, for a support call.
	Detail string
}

// Enroll classifies a failure from POST /b2b/stations/enroll.
func Enroll(err error) Result {
	if err == nil {
		return Result{Phase: status.PhaseReady, Retryable: false}
	}
	if r, ok := transport(err); ok {
		return r
	}

	var apiErr *agent.APIError
	if !errors.As(err, &apiErr) {
		return Result{
			Phase: status.PhaseWaiting, Retryable: true, Code: "unknown",
			Problem: "Serverdan tushunarsiz javob keldi.",
			Action:  "Bir necha daqiqadan so'ng o'zi qayta urinadi. Muammo davom etsa, station.log faylini yuboring.",
			Detail:  err.Error(),
		}
	}

	switch apiErr.Code {
	// "conflict" is what a backend older than this agent answers for the same
	// thing; both are kept so a new agent talking to an old server, or the
	// reverse, still says something useful.
	case "hwid_other_org", "conflict":
		// The blocking row lives in ANOTHER org, and nothing on this PC can
		// clear it: hwid_hash is a hash of the Windows MachineGuid, so
		// deleting station.json and station.key changes nothing. Only an
		// admin, in the other school's panel, can free it.
		return Result{
			Phase: status.PhaseBlocked, Retryable: false, Code: apiErr.Code,
			Problem: "Bu kompyuter allaqachon BOSHQA avtomaktabga ro'yxatdan o'tgan.",
			Action: "Admin panelda eski maktabni oching va shu PC ulanishini bekor qiling " +
				"(yoki \"Butunlay o'chirish\"), keyin shu faylni qaytadan ishga tushiring. " +
				"Bu faylni o'chirish yoki qayta yuklab olish yordam bermaydi.",
			Detail: apiErr.Error(),
		}
	case "not_found":
		return Result{
			Phase: status.PhaseBlocked, Retryable: false, Code: apiErr.Code,
			Problem: "Bu o'rnatish faylidagi kalit endi ishlamaydi (muddati tugagan yoki bekor qilingan).",
			Action: "Admin panelda shu maktabning o'rnatish faylini QAYTA yuklab oling va " +
				"yangisini ishga tushiring. Litsenziya muddati tugamaganini ham tekshiring.",
			Detail: apiErr.Error(),
		}
	case "no_license":
		return Result{
			Phase: status.PhaseBlocked, Retryable: false, Code: apiErr.Code,
			Problem: "Bu maktabda faol litsenziya yo'q.",
			Action:  "Admin panelda maktabga litsenziya qo'shing (o'rin soni va muddati bilan), keyin qayta urinib ko'ring.",
			Detail:  apiErr.Error(),
		}
	case "seats_exhausted", "code_exhausted":
		return Result{
			Phase: status.PhaseBlocked, Retryable: false, Code: apiErr.Code,
			Problem: "Litsenziyadagi barcha PC o'rinlari band.",
			Action: "Admin panelda ishlatilmayotgan bitta PC ulanishini bekor qiling yoki " +
				"litsenziyaga o'rin qo'shing, keyin qayta urinib ko'ring.",
			Detail: apiErr.Error(),
		}
	case "org_suspended":
		return Result{
			Phase: status.PhaseBlocked, Retryable: false, Code: apiErr.Code,
			Problem: "Bu avtomaktab vaqtincha to'xtatilgan.",
			Action:  "Admin panelda maktab holatini \"faol\" ga qaytaring.",
			Detail:  apiErr.Error(),
		}
	case "invalid":
		return Result{
			Phase: status.PhaseBlocked, Retryable: false, Code: apiErr.Code,
			Problem: "Server bu kompyuter yuborgan ma'lumotni qabul qilmadi.",
			Action:  "station.log faylini yuboring — bu dasturdagi xato, maktab tomonida tuzatib bo'lmaydi.",
			Detail:  apiErr.Error(),
		}
	case "rate_limited":
		return Result{
			Phase: status.PhaseWaiting, Retryable: true, Code: apiErr.Code,
			Problem: "Serverga juda ko'p so'rov yuborildi.",
			Action:  "Hech narsa qilish shart emas — bir necha daqiqadan so'ng o'zi davom etadi.",
			Detail:  apiErr.Error(),
		}
	default:
		return Result{
			Phase: status.PhaseWaiting, Retryable: true, Code: apiErr.Code,
			Problem: "Serverda kutilmagan xatolik.",
			Action:  "Bir necha daqiqadan so'ng o'zi qayta urinadi.",
			Detail:  apiErr.Error(),
		}
	}
}

// Token classifies a failure from the challenge/token pair.
//
// clockOffSeconds is how far this PC's clock is from the backend's, as
// measured from the server's Date header; 0 means unknown. It is consulted
// first because a wrong clock produces station_unauthorized -- the same opaque
// code as a revoked station -- and telling a school to re-enrol when the real
// problem is a dead CMOS battery burns a licence seat and fixes nothing.
func Token(err error, clockOffSeconds int64) Result {
	if err == nil {
		return Result{Phase: status.PhaseReady, Retryable: false}
	}
	if errors.Is(err, agent.ErrNotEnrolled) {
		return Result{
			Phase: status.PhaseBlocked, Retryable: false, Code: "not_enrolled",
			Problem: "Bu kompyuter hech qaysi avtomaktabga ro'yxatdan o'tmagan.",
			Action:  "Admin paneldan shu maktab uchun yuklab olingan .exe faylni ishga tushiring.",
			Detail:  err.Error(),
		}
	}
	if r, ok := transport(err); ok {
		return r
	}

	if errors.Is(err, agent.ErrStationUnauthorized) {
		// maxClockSkewSeconds mirrors the backend's stationClockSkew (2
		// minutes, backend/internal/b2b/station_auth.go). Anything past it
		// makes every signature look replayed no matter how correct it is.
		const maxClockSkewSeconds = 120
		if clockOffSeconds > maxClockSkewSeconds || clockOffSeconds < -maxClockSkewSeconds {
			return Result{
				Phase: status.PhaseBlocked, Retryable: true, Code: "clock",
				Problem: "Bu kompyuterning soati noto'g'ri — server bilan farqi 2 daqiqadan ko'p.",
				Action: "Windows'da sana va vaqtni to'g'rilang (vaqt mintaqasi ham), " +
					"yoki cmd'da: w32tm /resync /force. Soat to'g'rilangach hammasi o'zi ishlaydi.",
				Detail: err.Error(),
			}
		}
		return Result{
			Phase: status.PhaseBlocked, Retryable: true, Code: "station_unauthorized",
			Problem: "Server bu kompyuterning ro'yxatdan o'tganini tanimayapti.",
			Action: "Agar maktab o'chirilib qayta yaratilgan bo'lsa — shu .exe ni -reenroll bilan ishga tushiring. " +
				"Agar admin bu PCni ataylab uzgan bo'lsa — shundayligicha qoldiring.",
			Detail: err.Error(),
		}
	}
	return Result{
		Phase: status.PhaseWaiting, Retryable: true, Code: "unknown",
		Problem: "Server bilan aloqada xatolik.",
		Action:  "Bir necha daqiqadan so'ng o'zi qayta urinadi.",
		Detail:  err.Error(),
	}
}

// transport recognises failures that never reached the backend at all.
//
// These are reported as PhaseWaiting even when the cause is permanent (a
// firewall rule, a missing root certificate): the agent genuinely does keep
// retrying, and a classroom PC that boots before its router hits exactly this
// path every single morning. What changes is that the school is now told which
// kind of unreachable it is, instead of being shown a spinner.
func transport(err error) (Result, bool) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return Result{
			Phase: status.PhaseWaiting, Retryable: true, Code: "timeout",
			Problem: "Server javob bermadi (vaqt tugadi).",
			Action:  "Internet aloqasini tekshiring. Aloqa tiklansa dastur o'zi davom etadi.",
			Detail:  err.Error(),
		}, true
	}

	var urlErr *url.Error
	if !errors.As(err, &urlErr) {
		return Result{}, false
	}
	text := urlErr.Error()

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return Result{
			Phase: status.PhaseWaiting, Retryable: true, Code: "dns",
			Problem: "drivergo.uz manzili topilmadi (DNS ishlamayapti).",
			Action:  "Internet aloqasini va DNS sozlamalarini tekshiring. Aloqa tiklansa dastur o'zi davom etadi.",
			Detail:  text,
		}, true
	}
	// x509 messages are matched on text because Go 1.20's certificate errors
	// are not a single wrapped sentinel. This is the Windows 7 case worth
	// naming out loud: an un-updated machine can be missing the root that
	// signs drivergo.uz, and a badly wrong clock makes every valid
	// certificate look expired.
	if strings.Contains(text, "x509") || strings.Contains(text, "certificate") {
		return Result{
			Phase: status.PhaseWaiting, Retryable: true, Code: "tls",
			Problem: "Xavfsiz ulanish (TLS) o'rnatilmadi — sertifikat qabul qilinmadi.",
			Action: "Avval kompyuter sanasi va vaqtini to'g'rilang. To'g'ri bo'lsa, " +
				"Windows yangilanishlarini o'rnating (eski Windows 7 da ildiz sertifikatlari yetishmaydi).",
			Detail: text,
		}, true
	}
	if strings.Contains(text, "proxy") {
		return Result{
			Phase: status.PhaseWaiting, Retryable: true, Code: "proxy",
			Problem: "Tarmoq proksisi ulanishga ruxsat bermadi.",
			Action:  "Maktab tarmog'ida drivergo.uz ga (443-port) ruxsat bering.",
			Detail:  text,
		}, true
	}
	return Result{
		Phase: status.PhaseWaiting, Retryable: true, Code: "network",
		Problem: "Serverga ulanib bo'lmadi.",
		Action:  "Internet aloqasini tekshiring va drivergo.uz maktab tarmog'ida ochiq ekaniga ishonch hosil qiling.",
		Detail:  text,
	}, true
}
