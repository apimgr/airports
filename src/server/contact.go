package server

import (
	"container/list"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/config"
	dbpkg "github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/notify"
	"github.com/apimgr/airports/src/security/pgp"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	contactCaptchaLifetime = 10 * time.Minute
	contactCaptchaLimit    = 4096
	contactFormMaxBytes    = 256 << 10
)

var contactTokenState = struct {
	sync.Mutex
	used    map[string]time.Time
	reviews map[string]time.Time
}{used: make(map[string]time.Time), reviews: make(map[string]time.Time)}

var webhookFailureLogState = struct {
	sync.Mutex
	last map[string]time.Time
}{last: make(map[string]time.Time)}

type contactDuplicateEntry struct {
	key       string
	expiresAt time.Time
}

type webhookDelivery struct {
	target    notify.WebhookTarget
	message   notify.WebhookMessage
	requestID string
	clientIP  string
}

var contactAntispamState = struct {
	sync.Mutex
	byIP       map[string][]time.Time
	byEmail    map[string][]time.Time
	global     []time.Time
	strikes    map[string][]time.Time
	bans       map[string]time.Time
	duplicates map[string]*list.Element
	order      *list.List
}{
	byIP:       make(map[string][]time.Time),
	byEmail:    make(map[string][]time.Time),
	strikes:    make(map[string][]time.Time),
	bans:       make(map[string]time.Time),
	duplicates: make(map[string]*list.Element),
	order:      list.New(),
}

var contactMarkupPattern = regexp.MustCompile(`(?i)<\s*/?\s*[a-z!][^>]*(?:>|$)|&(?:lt|gt|#0*60|#0*62|#x0*3c|#x0*3e);|\[(?:/?(?:url|img|b)|url=)`)
var contactLinkPattern = regexp.MustCompile(`(?i)\b(?:https?://|www\.)[^\s<>"']+|\b[a-z0-9][a-z0-9.-]*\.[a-z]{2,}(?:/[^\s<>"']*)?`)

func (s *Server) newContactCaptcha() (string, string, error) {
	return s.newContactCaptchaForLanguage(i18n.DefaultLanguage)
}

func (s *Server) newContactCaptchaForLanguage(lang string) (string, string, error) {
	left, err := rand.Int(rand.Reader, big.NewInt(20))
	if err != nil {
		return "", "", err
	}
	right, err := rand.Int(rand.Reader, big.NewInt(20))
	if err != nil {
		return "", "", err
	}
	challengeIDBytes := make([]byte, 16)
	if _, err := rand.Read(challengeIDBytes); err != nil {
		return "", "", err
	}
	challengeID := hex.EncodeToString(challengeIDBytes)
	a, b := int(left.Int64())+1, int(right.Int64())+1
	operator := "+"
	answer := a + b
	if operation, err := rand.Int(rand.Reader, big.NewInt(2)); err != nil {
		return "", "", err
	} else if operation.Int64() == 1 {
		operator = "-"
		if b > a {
			a, b = b, a
		}
		answer = a - b
	}
	if s.db == nil {
		return "", "", fmt.Errorf("contact CAPTCHA storage is unavailable")
	}
	if err := s.ensureContactCaptchaSchema(); err != nil {
		return "", "", err
	}
	now := time.Now().UTC()
	if _, err := s.db.Exec(`DELETE FROM captcha_challenges WHERE expires_at <= ?`, now.Unix()); err != nil {
		return "", "", err
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM captcha_challenges`).Scan(&count); err != nil {
		return "", "", err
	}
	if count >= contactCaptchaLimit {
		return "", "", fmt.Errorf("contact CAPTCHA challenge limit reached")
	}
	answerHash := sha256.Sum256([]byte(strconv.Itoa(answer)))
	if _, err := s.db.Exec(`INSERT INTO captcha_challenges(id, answer_hash, expires_at) VALUES(?, ?, ?)`, challengeID, hex.EncodeToString(answerHash[:]), now.Add(contactCaptchaLifetime).Unix()); err != nil {
		return "", "", err
	}
	question := fmt.Sprintf(i18n.TranslateText(lang, "contact.captcha_question"), a, operator, b)
	return question, challengeID, nil
}

func (s *Server) consumeContactCaptcha(challengeID, submitted string) bool {
	if s.db == nil || s.ensureContactCaptchaSchema() != nil {
		return false
	}
	var valid bool
	err := dbpkg.Transaction(context.Background(), s.db, func(tx *dbpkg.TransactionHandle) error {
		var answerHashText string
		var expiresAt int64
		if err := tx.QueryRow(`SELECT answer_hash, expires_at FROM captcha_challenges WHERE id = ?`, challengeID).Scan(&answerHashText, &expiresAt); err != nil || time.Now().UTC().Unix() >= expiresAt {
			return nil
		}
		result, err := tx.Exec(`DELETE FROM captcha_challenges WHERE id = ?`, challengeID)
		if err != nil {
			return err
		}
		if affected, err := result.RowsAffected(); err != nil {
			return err
		} else if affected != 1 {
			return nil
		}
		answerHash := sha256.Sum256([]byte(strings.TrimSpace(submitted)))
		valid = subtle.ConstantTimeCompare([]byte(answerHashText), []byte(hex.EncodeToString(answerHash[:]))) == 1
		return nil
	})
	return err == nil && valid
}

func (s *Server) ensureContactCaptchaSchema() error {
	if s.db == nil {
		return fmt.Errorf("contact CAPTCHA storage is unavailable")
	}
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS captcha_challenges (
		id TEXT PRIMARY KEY,
		answer_hash TEXT NOT NULL,
		expires_at INTEGER NOT NULL
	)`)
	return err
}

// CleanupExpiredContactChallenges removes expired one-time CAPTCHA challenges.
func CleanupExpiredContactChallenges(db dbpkg.Handle, now time.Time) error {
	if db == nil {
		return fmt.Errorf("contact CAPTCHA storage is unavailable")
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS captcha_challenges (
		id TEXT PRIMARY KEY,
		answer_hash TEXT NOT NULL,
		expires_at INTEGER NOT NULL
	)`); err != nil {
		return err
	}
	_, err := db.Exec(`DELETE FROM captcha_challenges WHERE expires_at <= ?`, now.UTC().Unix())
	return err
}

func contactFormFields(kind string) []map[string]interface{} {
	field := func(name, label, inputType string, required bool, maxLength int) map[string]interface{} {
		var limit interface{}
		if maxLength > 0 {
			limit = maxLength
		}
		return map[string]interface{}{"name": name, "label": label, "type": inputType, "required": required, "max_length": limit, "required_when": nil, "max_length_when": nil}
	}
	fields := []map[string]interface{}{field("name", "contact.name", "text", true, 100), field("email", "contact.email", "email", true, 254)}
	switch kind {
	case "abuse":
		reportedURL := field("reported_url", "contact.reported_url", "url", false, 2048)
		reportedURL["required_when"] = map[string]interface{}{"field": "type", "in": []string{"abuse", "dmca"}}
		fields = append(fields, reportedURL, field("reason", "contact.reason", "select", true, 0))
		fields[len(fields)-1]["options"] = []string{"illegal_content", "terms_violation", "harassment", "malware_phishing", "other"}
	case "dmca":
		reportedURL := field("reported_url", "contact.reported_url", "url", false, 2048)
		reportedURL["required_when"] = map[string]interface{}{"field": "type", "in": []string{"abuse", "dmca"}}
		fields = append(fields, reportedURL, field("copyrighted_work", "contact.copyrighted_work", "textarea", true, 2000), field("good_faith", "contact.good_faith", "checkbox", true, 0), field("accuracy", "contact.accuracy", "checkbox", true, 0), field("signature", "contact.signature", "text", true, 100))
	case "privacy":
		fields = append(fields, field("request_kind", "contact.request_kind", "select", true, 0))
		fields[len(fields)-1]["options"] = []string{"access", "export", "deletion", "correction", "opt_out", "other"}
	case "security":
		fields = append(fields,
			field("component", "contact.component", "text", true, 255),
			field("endpoint", "contact.endpoint", "text", false, 2048),
			field("severity", "contact.severity", "select", true, 0),
			field("summary", "contact.summary", "text", true, 200),
			field("steps_to_reproduce", "contact.steps_to_reproduce", "textarea", true, 10000),
			field("impact", "contact.impact", "textarea", true, 5000),
			field("suggested_fix", "contact.suggested_fix", "textarea", false, 5000),
			field("cve_requested", "contact.cve_requested", "checkbox", false, 0),
			field("timeline_days", "contact.timeline_days", "number", false, 0),
			field("credit_preference", "contact.credit_preference", "select", true, 0),
			field("credit_handle", "contact.credit_handle", "text", false, 100),
			field("researcher_gpg", "contact.researcher_gpg", "textarea", false, 16384),
			field("disclosure_agreement", "contact.disclosure_agreement", "checkbox", true, 0),
		)
		fields[4]["options"] = []string{"critical", "high", "medium", "low", "informational"}
		fields[11]["options"] = []string{"name", "handle", "no", "anon"}
		fields[12]["required_when"] = map[string]string{"field": "credit_preference", "equals": "handle"}
		fields[13]["max_length_when"] = []map[string]interface{}{{"prefix": "https://", "max_length": 2048}}
	default:
		return nil
	}
	if kind != "security" {
		fields = append(fields, field("subject", "contact.subject", "text", true, 200), field("message", "contact.message", "textarea", true, 5000))
	}
	return fields
}

func contactAllowedTypes(securityMode bool) []string {
	if securityMode {
		return []string{"security"}
	}
	return []string{"abuse", "dmca", "privacy"}
}

func contactFormToken(s *Server, now time.Time) (string, error) {
	secret, err := s.installationSecret()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	issued := strconv.FormatInt(now.Unix(), 10)
	nonceText := hex.EncodeToString(nonce)
	payload := "contact-form|" + issued + "|" + nonceText
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	return issued + "." + nonceText + "." + hex.EncodeToString(mac.Sum(nil)), nil
}

func verifyContactFormToken(s *Server, token string, now time.Time, consume bool) bool {
	return verifyContactFormTokenReason(s, token, now, consume) == ""
}

func verifyContactFormTokenReason(s *Server, token string, now time.Time, consume bool) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || len(parts[1]) != 32 || len(parts[2]) != 64 {
		return "fill_time"
	}
	issued, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "fill_time"
	}
	age := now.Sub(time.Unix(issued, 0))
	settings := contactAntispamConfig(s)
	if age < time.Duration(settings.MinFillSeconds)*time.Second || settings.MaxAgeSeconds > 0 && age > time.Duration(settings.MaxAgeSeconds)*time.Second {
		return "fill_time"
	}
	expected, err := hex.DecodeString(parts[2])
	if err != nil || !verifyInstallationHMAC(s, "contact-form|"+parts[0]+"|"+parts[1], expected, now) {
		return "fill_time"
	}
	if !consume {
		return ""
	}
	var expires time.Time
	if settings.MaxAgeSeconds > 0 {
		expires = time.Unix(issued, 0).Add(time.Duration(settings.MaxAgeSeconds) * time.Second)
	}
	if !consumeContactNonce(parts[1], expires, now) {
		return "replay"
	}
	return ""
}

func contactReviewToken(s *Server, values map[string]string, now time.Time) (string, error) {
	secret, err := s.installationSecret()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	issued := strconv.FormatInt(now.Unix(), 10)
	nonceText := hex.EncodeToString(nonce)
	fieldsHash := contactFieldsHash(values)
	flags := ""
	if values["website"] != "" && contactAntispamConfig(s).Honeypot {
		flags = "d"
	}
	payload := "contact-review|" + issued + "|" + nonceText + "|" + flags + "|" + fieldsHash
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(payload))
	token := issued + "." + nonceText + "." + flags + "." + hex.EncodeToString(mac.Sum(nil))
	contactTokenState.Lock()
	pruneContactNonces(now)
	if len(contactTokenState.reviews) >= 50000 {
		contactTokenState.Unlock()
		return "", fmt.Errorf("contact review limit reached")
	}
	maxAgeSeconds := contactAntispamConfig(s).ReviewMaxSeconds
	var expires time.Time
	if maxAgeSeconds > 0 {
		expires = time.Unix(now.Unix(), 0).Add(time.Duration(maxAgeSeconds) * time.Second)
	}
	contactTokenState.reviews[nonceText] = expires
	contactTokenState.Unlock()
	return token, nil
}

func consumeContactReviewToken(s *Server, token string, values map[string]string, now time.Time) bool {
	return consumeContactReviewTokenReason(s, token, values, now) == ""
}

func consumeContactReviewTokenReason(s *Server, token string, values map[string]string, now time.Time) string {
	parts := strings.Split(token, ".")
	if len(parts) != 4 || len(parts[1]) != 32 || (parts[2] != "" && parts[2] != "d") || len(parts[3]) != 64 {
		return "review"
	}
	issued, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return "review"
	}
	age := now.Sub(time.Unix(issued, 0))
	settings := contactAntispamConfig(s)
	if age < time.Duration(settings.ReviewMinSeconds)*time.Second || settings.ReviewMaxSeconds > 0 && age > time.Duration(settings.ReviewMaxSeconds)*time.Second {
		return "review"
	}
	flags := ""
	if values["website"] != "" && contactAntispamConfig(s).Honeypot {
		flags = "d"
	}
	if parts[2] != flags {
		return "review"
	}
	expected, err := hex.DecodeString(parts[3])
	if err != nil || !verifyInstallationHMAC(s, "contact-review|"+parts[0]+"|"+parts[1]+"|"+parts[2]+"|"+contactFieldsHash(values), expected, now) {
		return "review"
	}
	contactTokenState.Lock()
	defer contactTokenState.Unlock()
	pruneContactNonces(now)
	if _, ok := contactTokenState.reviews[parts[1]]; !ok {
		return "replay"
	}
	delete(contactTokenState.reviews, parts[1])
	return ""
}

func verifyInstallationHMAC(s *Server, payload string, provided []byte, now time.Time) bool {
	current, err := s.installationSecret()
	if err != nil {
		return false
	}
	secrets := [][]byte{current}
	if previous, ok := s.previousInstallationSecret(now); ok {
		secrets = append(secrets, previous)
	}
	valid := 0
	for _, secret := range secrets {
		mac := hmac.New(sha256.New, secret)
		_, _ = mac.Write([]byte(payload))
		valid |= subtle.ConstantTimeCompare(provided, mac.Sum(nil))
	}
	return valid == 1
}

func contactFieldsHash(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		switch key {
		case "csrf_token", "captcha_id", "captcha_answer", "captcha", "captcha_response", "g-recaptcha-response", "h-captcha-response", "form_token", "step", "review_token", "confirmed":
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var canonical strings.Builder
	for _, key := range keys {
		canonical.WriteString(key)
		canonical.WriteByte('=')
		canonical.WriteString(strings.Join(strings.Fields(strings.TrimSpace(values[key])), " "))
		canonical.WriteByte('\n')
	}
	digest := sha256.Sum256([]byte(canonical.String()))
	return hex.EncodeToString(digest[:])
}

func consumeContactNonce(nonce string, expires, now time.Time) bool {
	contactTokenState.Lock()
	defer contactTokenState.Unlock()
	pruneContactNonces(now)
	if _, exists := contactTokenState.used[nonce]; exists || len(contactTokenState.used) >= 50000 {
		return false
	}
	contactTokenState.used[nonce] = expires
	return true
}

func pruneContactNonces(now time.Time) {
	for nonce, expiry := range contactTokenState.used {
		if !expiry.IsZero() && !now.Before(expiry) {
			delete(contactTokenState.used, nonce)
		}
	}
	for nonce, expiry := range contactTokenState.reviews {
		if !expiry.IsZero() && !now.Before(expiry) {
			delete(contactTokenState.reviews, nonce)
		}
	}
}

func contactAntispamConfig(s *Server) config.ContactAntispamConfig {
	if s == nil || s.config == nil {
		return config.ContactAntispamConfig{MinFillSeconds: 3, MaxAgeSeconds: 43200, PerIP: 5, PerEmail: 3, Global: 60, Window: "1h", MaxLinks: 10, RejectMarkup: true, Strikes: 3, BanHours: 24, ReviewMinSeconds: 2, ReviewMaxSeconds: 900, DuplicateWindowHours: 24}
	}
	return s.config.Server.Pages.Contact.Antispam
}

func contactWindow(settings config.ContactAntispamConfig) time.Duration {
	window, err := time.ParseDuration(strings.TrimSpace(settings.Window))
	if err != nil || window <= 0 {
		return time.Hour
	}
	return window
}

func contactClientIP(r *http.Request) string {
	if r == nil {
		return "unknown"
	}
	if ip := strings.TrimSpace(middleware.GetClientIP(r.Context())); ip != "" {
		return ip
	}
	ip := strings.TrimSpace(r.RemoteAddr)
	if host, _, err := net.SplitHostPort(ip); err == nil {
		ip = host
	}
	if net.ParseIP(ip) == nil {
		return "unknown"
	}
	return ip
}

func contactIdentity(value string) string {
	digest := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(digest[:])
}

func validContactBlockReason(reason string) bool {
	switch reason {
	case "shape", "honeypot", "fill_time", "captcha", "rate_ip", "rate_email", "rate_global", "content", "markup", "duplicate", "banned", "origin", "replay", "review":
		return true
	default:
		return false
	}
}

func (s *Server) recordContactSpamBlock(r *http.Request, reason string, strike bool) {
	if !validContactBlockReason(reason) {
		return
	}
	clientIP := contactClientIP(r)
	slog.Info("contact.spam_blocked", "reason", reason, "client_ip", clientIP)
	if s.metrics != nil {
		s.metrics.recordContactBlocked(reason)
	}
	if strike {
		recordContactStrike(clientIP, contactAntispamConfig(s), time.Now(), s.isOverlayRequest(r))
	}
}

func contactRetryAfter(remaining time.Duration) string {
	seconds := int(math.Ceil(remaining.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return strconv.Itoa(seconds)
}

func (s *Server) contactBanRemaining(r *http.Request, now time.Time) time.Duration {
	if s.isOverlayRequest(r) {
		return 0
	}
	identity := contactIdentity(contactClientIP(r))
	contactAntispamState.Lock()
	defer contactAntispamState.Unlock()
	until, exists := contactAntispamState.bans[identity]
	if !exists {
		return 0
	}
	if !now.Before(until) {
		delete(contactAntispamState.bans, identity)
		return 0
	}
	return until.Sub(now)
}

func recordContactStrike(clientIP string, settings config.ContactAntispamConfig, now time.Time, overlay bool) {
	if overlay || settings.Strikes <= 0 || settings.BanHours <= 0 {
		return
	}
	identity := contactIdentity(clientIP)
	window := contactWindow(settings)
	cutoff := now.Add(-window)
	contactAntispamState.Lock()
	defer contactAntispamState.Unlock()
	contactAntispamState.strikes[identity] = pruneContactTimes(contactAntispamState.strikes[identity], cutoff)
	contactAntispamState.strikes[identity] = append(contactAntispamState.strikes[identity], now)
	if len(contactAntispamState.strikes) > 10000 {
		for key, times := range contactAntispamState.strikes {
			if len(times) == 0 {
				delete(contactAntispamState.strikes, key)
			}
		}
		if len(contactAntispamState.strikes) > 10000 {
			for key := range contactAntispamState.strikes {
				if key != identity {
					delete(contactAntispamState.strikes, key)
					break
				}
			}
		}
	}
	if len(contactAntispamState.strikes[identity]) >= settings.Strikes {
		contactAntispamState.bans[identity] = now.Add(time.Duration(settings.BanHours) * time.Hour)
		delete(contactAntispamState.strikes, identity)
		for len(contactAntispamState.bans) > 10000 {
			for key := range contactAntispamState.bans {
				if key != identity {
					delete(contactAntispamState.bans, key)
					break
				}
			}
		}
	}
}

func pruneContactTimes(values []time.Time, cutoff time.Time) []time.Time {
	firstLive := 0
	for firstLive < len(values) && values[firstLive].Before(cutoff) {
		firstLive++
	}
	return values[firstLive:]
}

func (s *Server) checkContactSubmissionRate(r *http.Request, values map[string]string, now time.Time, record bool) (string, time.Duration) {
	settings := contactAntispamConfig(s)
	window := contactWindow(settings)
	cutoff := now.Add(-window)
	ip := contactIdentity(contactClientIP(r))
	email := contactIdentity(values["email"])
	checkBucket := func(events []time.Time, limit int) (int, []time.Time, time.Duration) {
		events = pruneContactTimes(events, cutoff)
		if limit > 0 && len(events) >= limit {
			return len(events), events, events[0].Add(window).Sub(now)
		}
		return len(events), events, 0
	}
	contactAntispamState.Lock()
	defer contactAntispamState.Unlock()
	if !s.isOverlayRequest(r) {
		ipCount, ipEvents, ipWait := checkBucket(contactAntispamState.byIP[ip], settings.PerIP)
		contactAntispamState.byIP[ip] = ipEvents
		if settings.PerIP > 0 && ipCount >= settings.PerIP {
			return "rate_ip", ipWait
		}
	}
	emailCount, emailEvents, emailWait := checkBucket(contactAntispamState.byEmail[email], settings.PerEmail)
	contactAntispamState.byEmail[email] = emailEvents
	if settings.PerEmail > 0 && emailCount >= settings.PerEmail {
		return "rate_email", emailWait
	}
	globalCount, globalEvents, globalWait := checkBucket(contactAntispamState.global, settings.Global)
	contactAntispamState.global = globalEvents
	if settings.Global > 0 && globalCount >= settings.Global {
		return "rate_global", globalWait
	}
	if record {
		if !s.isOverlayRequest(r) && settings.PerIP > 0 {
			contactAntispamState.byIP[ip] = append(contactAntispamState.byIP[ip], now)
		}
		if settings.PerEmail > 0 {
			contactAntispamState.byEmail[email] = append(contactAntispamState.byEmail[email], now)
		}
		if settings.Global > 0 {
			contactAntispamState.global = append(contactAntispamState.global, now)
		}
		boundContactRateState(ip, email)
	}
	return "", 0
}

func boundContactRateState(ip, email string) {
	for len(contactAntispamState.byIP) > 10000 {
		for key := range contactAntispamState.byIP {
			if key != ip {
				delete(contactAntispamState.byIP, key)
				break
			}
		}
	}
	for len(contactAntispamState.byEmail) > 10000 {
		for key := range contactAntispamState.byEmail {
			if key != email {
				delete(contactAntispamState.byEmail, key)
				break
			}
		}
	}
	if len(contactAntispamState.global) > 100000 {
		contactAntispamState.global = contactAntispamState.global[len(contactAntispamState.global)-100000:]
	}
}

func contactDuplicateKey(values map[string]string) string {
	parts := []string{values["email"], values["subject"], values["message"]}
	for index, part := range parts {
		parts[index] = strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(part)), " "))
	}
	digest := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(digest[:])
}

func recordContactDuplicate(values map[string]string, settings config.ContactAntispamConfig, now time.Time) bool {
	if settings.DuplicateWindowHours <= 0 {
		return false
	}
	key := contactDuplicateKey(values)
	expires := now.Add(time.Duration(settings.DuplicateWindowHours) * time.Hour)
	contactAntispamState.Lock()
	defer contactAntispamState.Unlock()
	for element := contactAntispamState.order.Back(); element != nil; {
		previous := element.Prev()
		entry := element.Value.(contactDuplicateEntry)
		if !now.Before(entry.expiresAt) {
			delete(contactAntispamState.duplicates, entry.key)
			contactAntispamState.order.Remove(element)
		}
		element = previous
	}
	if element, exists := contactAntispamState.duplicates[key]; exists {
		entry := element.Value.(contactDuplicateEntry)
		if now.Before(entry.expiresAt) {
			contactAntispamState.order.MoveToFront(element)
			return true
		}
		contactAntispamState.order.Remove(element)
		delete(contactAntispamState.duplicates, key)
	}
	element := contactAntispamState.order.PushFront(contactDuplicateEntry{key: key, expiresAt: expires})
	contactAntispamState.duplicates[key] = element
	if contactAntispamState.order.Len() > 10000 {
		oldest := contactAntispamState.order.Back()
		entry := oldest.Value.(contactDuplicateEntry)
		delete(contactAntispamState.duplicates, entry.key)
		contactAntispamState.order.Remove(oldest)
	}
	return false
}

func (s *Server) rejectContactSpam(w http.ResponseWriter, r *http.Request, values map[string]string, reason string, strike bool, status int, code, message string) {
	s.recordContactSpamBlock(r, reason, strike)
	s.contactFailure(w, r, values, status, code, message)
}

func (s *Server) handleContactSubmission(w http.ResponseWriter, r *http.Request) {
	if !s.config.Server.Pages.Contact.Enabled {
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, contactFormMaxBytes)
	values, err := contactValuesFromRequest(r)
	if err != nil {
		s.rejectContactSpam(w, r, nil, "shape", false, http.StatusBadRequest, "VALIDATION_FAILED", "The contact form is invalid or too large.")
		return
	}
	r.Form = make(url.Values, len(values))
	r.PostForm = make(url.Values, len(values))
	for key, value := range values {
		r.Form.Set(key, value)
		r.PostForm.Set(key, value)
	}
	if !contactShapeValid(values) {
		s.rejectContactSpam(w, r, values, "shape", false, http.StatusBadRequest, "VALIDATION_FAILED", "The contact form contains an invalid or unexpected field.")
		return
	}
	if remaining := s.contactBanRemaining(r, time.Now()); remaining > 0 {
		s.recordContactSpamBlock(r, "banned", false)
		w.Header().Set("Retry-After", contactRetryAfter(remaining))
		s.contactFailure(w, r, values, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests")
		return
	}
	if contactBrowserPost(r) && !contactOriginValid(r) {
		s.rejectContactSpam(w, r, values, "origin", true, http.StatusBadRequest, "VALIDATION_FAILED", "Please submit the form from this site.")
		return
	}
	kind := strings.ToLower(strings.TrimSpace(values["type"]))
	securityID := strings.TrimSpace(values["security_id"])
	securityMode := securityID != "" && s.validSecurityID(securityID, time.Now())
	if securityID != "" && !securityMode {
		s.logInvalidSecurityID(r, securityID)
	}
	if !contactTypeAllowed(kind, securityMode) {
		s.rejectContactSpam(w, r, values, "shape", false, http.StatusBadRequest, "VALIDATION_FAILED", "Choose a valid report type.")
		return
	}
	if values["step"] == "edit" {
		s.renderContactStep(w, r, values, "edit", "", "")
		return
	}
	if values["step"] == "" {
		values["step"] = "submit"
	}
	if values["step"] != "submit" && values["step"] != "confirm" {
		s.rejectContactSpam(w, r, values, "shape", false, http.StatusBadRequest, "VALIDATION_FAILED", "Choose a valid contact step.")
		return
	}
	if err := validateContactFields(values, kind); err != nil {
		s.rejectContactSpam(w, r, values, "shape", false, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}
	now := time.Now()
	if values["step"] == "submit" {
		if reason := verifyContactFormTokenReason(s, values["form_token"], now, true); reason != "" {
			s.rejectContactSpam(w, r, values, reason, reason == "replay", http.StatusBadRequest, "VALIDATION_FAILED", "Please wait a moment and submit again.")
			return
		}
		honeypot := values["website"] != "" && contactAntispamConfig(s).Honeypot
		if !honeypot {
			if !contactCaptchaValid(s, r, values) {
				s.rejectContactSpam(w, r, values, "captcha", true, http.StatusBadRequest, "VALIDATION_FAILED", "The captcha response is invalid or expired.")
				return
			}
			if reason, err := validateContactContentReason(s, r, values, kind, contactAntispamConfig(s)); err != nil {
				s.rejectContactSpam(w, r, values, reason, true, http.StatusBadRequest, "VALIDATION_FAILED", "Your message could not be sent.")
				return
			}
		}
		if reason, remaining := s.checkContactSubmissionRate(r, values, now, false); reason != "" {
			s.recordContactSpamBlock(r, reason, false)
			w.Header().Set("Retry-After", contactRetryAfter(remaining))
			s.contactFailure(w, r, values, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests")
			return
		}
		if honeypot {
			s.recordContactSpamBlock(r, "honeypot", true)
		}
		reviewToken, err := contactReviewToken(s, values, now)
		if err != nil {
			s.contactFailure(w, r, values, http.StatusServiceUnavailable, "CONTACT_UNAVAILABLE", "The contact form is unavailable.")
			return
		}
		s.renderContactStep(w, r, values, "review", reviewToken, "")
		return
	}
	if values["confirmed"] != "true" {
		s.rejectContactSpam(w, r, values, "review", false, http.StatusBadRequest, "VALIDATION_FAILED", "Please read the details and confirm.")
		return
	}
	if reason := consumeContactReviewTokenReason(s, values["review_token"], values, now); reason != "" {
		s.rejectContactSpam(w, r, values, reason, reason == "replay", http.StatusBadRequest, "VALIDATION_FAILED", "Please read the details and confirm.")
		return
	}
	if reason, remaining := s.checkContactSubmissionRate(r, values, now, true); reason != "" {
		s.recordContactSpamBlock(r, reason, false)
		w.Header().Set("Retry-After", contactRetryAfter(remaining))
		s.contactFailure(w, r, values, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests")
		return
	}
	if values["website"] != "" && contactAntispamConfig(s).Honeypot {
		s.recordContactSpamBlock(r, "honeypot", false)
		s.renderContactStep(w, r, values, "success", "", contactSuccessMessage(s))
		return
	}
	if kind != "security" && recordContactDuplicate(values, contactAntispamConfig(s), now) {
		s.recordContactSpamBlock(r, "duplicate", true)
		s.renderContactStep(w, r, values, "success", "", contactSuccessMessage(s))
		return
	}
	if kind == "security" {
		if err := s.submitSecurityReport(w, r, values["name"], values["email"]); err != nil {
			s.contactFailure(w, r, values, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
			return
		}
		return
	}
	contactRole, webhookRole := "abuse", "abuse"
	if kind == "privacy" {
		contactRole = "dpo"
		if strings.TrimSpace(s.config.Server.Contact.DPO.Email) == "" {
			webhookRole = "general"
		} else {
			webhookRole = ""
		}
	}
	recipient := s.config.Server.Contact.ResolveEmail(contactRole)
	canEmail := recipient != "" && notify.CanSend(s.config)
	webhooks := s.contactWebhookTargets(webhookRole)
	overlayRequest := s.isTorRequest(r) || s.isI2PRequest(r)
	configuredWebhookTargets := len(webhooks) > 0
	if overlayRequest {
		webhooks = nil
	}
	if !canEmail && len(webhooks) == 0 {
		messageKey := "contact.delivery_unavailable"
		if overlayRequest && configuredWebhookTargets {
			messageKey = "contact.overlay_delivery_unavailable"
		}
		message := i18n.TranslateText(i18n.FromContext(r.Context()), messageKey)
		s.contactFailure(w, r, values, http.StatusServiceUnavailable, "CONTACT_UNAVAILABLE", message)
		return
	}
	delivery := map[string]string{"sender_name": values["name"], "contact_subject": values["subject"], "contact_message": contactPlainTextBody(values, kind)}
	var emailErr error
	if canEmail {
		emailErr = notify.SendWithReplyTo(s.config, s.configDir, "contact_submission", recipient, values["email"], delivery)
	}
	if emailErr != nil {
		slog.Error("contact form email delivery failed", "request_id", middleware.GetReqID(r.Context()), "error", emailErr)
	}
	if len(webhooks) > 0 {
		message := notify.WebhookMessage{
			Event: "contact.report_received", Subject: "[" + s.config.Server.Branding.Title + " contact] " + values["subject"],
			Body:     "Sender name: " + values["name"] + "\nSender email: " + values["email"] + "\n" + contactPlainTextBody(values, kind),
			Severity: "info", Timestamp: time.Now().UTC(), ProjectName: s.config.Server.Branding.Title,
			ProjectVersion: Version, AppURL: strings.TrimSuffix(RequestBaseURL(r), "/"), Component: "contact",
		}
		s.dispatchContactWebhooks(r, webhooks, message)
	}
	if emailErr != nil && len(webhooks) == 0 {
		s.contactFailure(w, r, values, http.StatusBadGateway, "CONTACT_DELIVERY_FAILED", i18n.TranslateText(i18n.FromContext(r.Context()), "contact.delivery_failed"))
		return
	}
	s.renderContactStep(w, r, values, "success", "", contactSuccessMessage(s))
}

func (s *Server) dispatchContactWebhooks(r *http.Request, targets []notify.WebhookTarget, message notify.WebhookMessage) {
	requestID := middleware.GetReqID(r.Context())
	clientIP := middleware.GetClientIP(r.Context())
	s.webhookMu.Lock()
	if s.webhookStopped {
		s.webhookMu.Unlock()
		return
	}
	if s.webhookQueue == nil {
		s.webhookCtx, s.webhookCancel = context.WithCancel(context.Background())
		s.webhookQueue = make(chan webhookDelivery, 128)
		for i := 0; i < 4; i++ {
			s.webhookWG.Add(1)
			go s.runContactWebhookWorker()
		}
	}
	queue, ctx := s.webhookQueue, s.webhookCtx
	s.webhookMu.Unlock()
	for _, target := range targets {
		job := webhookDelivery{target: target, message: message, requestID: requestID, clientIP: clientIP}
		select {
		case queue <- job:
		case <-ctx.Done():
			return
		}
	}
}

func (s *Server) runContactWebhookWorker() {
	defer s.webhookWG.Done()
	for {
		s.webhookMu.Lock()
		queue, ctx := s.webhookQueue, s.webhookCtx
		s.webhookMu.Unlock()
		select {
		case <-ctx.Done():
			return
		case job := <-queue:
			_, _ = notify.DeliverWebhookWithRetry(ctx, job.target, job.message, func(err error) {
				if allowWebhookFailureLog(job.clientIP, time.Now()) {
					slog.Error("notify.webhook_failed", "request_id", job.requestID, "client_ip", job.clientIP, "role", job.target.Role, "transport", job.target.Transport, "error", err)
				}
			})
		}
	}
}

func allowWebhookFailureLog(clientIP string, now time.Time) bool {
	if clientIP == "" {
		clientIP = "<unknown>"
	}
	webhookFailureLogState.Lock()
	defer webhookFailureLogState.Unlock()
	for ip, last := range webhookFailureLogState.last {
		if now.Sub(last) >= time.Minute {
			delete(webhookFailureLogState.last, ip)
		}
	}
	if last, exists := webhookFailureLogState.last[clientIP]; exists && now.Sub(last) < time.Minute {
		return false
	}
	if len(webhookFailureLogState.last) >= 10000 {
		var oldestIP string
		var oldest time.Time
		for ip, last := range webhookFailureLogState.last {
			if oldestIP == "" || last.Before(oldest) {
				oldestIP, oldest = ip, last
			}
		}
		delete(webhookFailureLogState.last, oldestIP)
	}
	webhookFailureLogState.last[clientIP] = now
	return true
}

func (s *Server) contactWebhookTargets(role string) []notify.WebhookTarget {
	if role == "" {
		return nil
	}
	contact := s.config.Server.Contact
	candidates := []config.ContactRoleConfig{}
	switch role {
	case "admin":
		candidates = append(candidates, contact.Admin)
	case "security":
		candidates = append(candidates, contact.Security, contact.Admin)
	case "abuse":
		candidates = append(candidates, contact.Abuse, contact.General, contact.Admin)
	case "general":
		candidates = append(candidates, contact.General, contact.Admin)
	default:
		return nil
	}
	transports := make(map[string]struct{})
	for _, candidate := range candidates {
		for transport, endpoint := range candidate.Webhooks {
			if strings.HasSuffix(transport, "_secret") || strings.TrimSpace(endpoint) == "" {
				continue
			}
			transports[transport] = struct{}{}
		}
	}
	ordered := make([]string, 0, len(transports))
	for transport := range transports {
		ordered = append(ordered, transport)
	}
	sort.Strings(ordered)
	targets := make([]notify.WebhookTarget, 0, len(ordered))
	for _, transport := range ordered {
		endpoint := contact.ResolveWebhook(role, transport)
		secret := contact.ResolveWebhookSigningSecret(role, transport)
		if endpoint == "" || secret == "" {
			continue
		}
		targets = append(targets, notify.WebhookTarget{Role: role, Transport: transport, URL: endpoint, Secret: secret})
	}
	return targets
}

func (s *Server) contactWebhookTargetsForRole(role string, configRole config.ContactRoleConfig) []notify.WebhookTarget {
	transports := make([]string, 0, len(configRole.Webhooks))
	for transport, endpoint := range configRole.Webhooks {
		if strings.HasSuffix(transport, "_secret") || strings.TrimSpace(endpoint) == "" {
			continue
		}
		transports = append(transports, transport)
	}
	sort.Strings(transports)
	targets := make([]notify.WebhookTarget, 0, len(transports))
	for _, transport := range transports {
		secret := strings.TrimSpace(configRole.Webhooks[transport+"_secret"])
		if secret == "" {
			continue
		}
		targets = append(targets, notify.WebhookTarget{
			Role: role, Transport: transport, URL: strings.TrimSpace(configRole.Webhooks[transport]), Secret: secret,
		})
	}
	return targets
}

func (s *Server) contactFailure(w http.ResponseWriter, r *http.Request, values map[string]string, status int, code, message string) {
	kind, securityID := "", ""
	if values != nil {
		kind, securityID = values["type"], values["security_id"]
	}
	securityMode := kind == "security" && securityID != "" && s.validSecurityID(securityID, time.Now())
	if !contactBrowserPost(r) {
		details := map[string]interface{}{}
		if contactTypeAllowed(kind, securityMode) {
			if question, challengeID, err := s.newContactCaptchaForLanguage(i18n.FromContext(r.Context())); err == nil {
				formToken, tokenErr := contactFormToken(s, time.Now())
				if tokenErr == nil {
					provider := s.contactCaptchaProvider(r)
					if provider == "" || provider == "simple" {
						details["captcha"] = map[string]string{"provider": "simple", "captcha_id": challengeID, "question": question}
					} else {
						details["captcha"] = map[string]interface{}{"provider": provider, "site_key": s.config.Server.Pages.Contact.CaptchaSiteKey, "fallback": map[string]string{"provider": "simple", "captcha_id": challengeID, "question": question}}
					}
					details["form_token"] = formToken
				}
			}
		}
		if id := middleware.GetReqID(r.Context()); id != "" {
			details["request_id"] = id
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if err := encodeJSON(w, ErrorResponse{OK: false, Error: code, Message: message, Details: details}); err != nil {
			slog.Error("contact error response failed", "error", err)
		}
		return
	}
	showChooser := !contactTypeAllowed(kind, securityMode)
	data, err := s.contactPageData(r, kind, securityMode, securityID, showChooser)
	if err != nil {
		s.writeErrorJSON(w, r, status, code, message)
		return
	}
	if values != nil {
		fields, _ := data["FormFields"].([]map[string]interface{})
		for _, field := range fields {
			if name, ok := field["name"].(string); ok {
				field["value"] = values[name]
			}
		}
	}
	data["ValidationMessage"] = message
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	if err := s.executeTemplateStatus(w, r, "server_contact", status, data); err != nil {
		s.renderErrorPage(w, r, http.StatusInternalServerError)
	}
}

func contactValuesFromRequest(r *http.Request) (map[string]string, error) {
	values := make(map[string]string)
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		var raw map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			return nil, err
		}
		for key, value := range raw {
			switch typed := value.(type) {
			case string:
				values[key] = typed
			case bool:
				values[key] = strconv.FormatBool(typed)
			case nil:
				values[key] = ""
			default:
				return nil, fmt.Errorf("contact fields must be scalar values")
			}
		}
		return values, nil
	}
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	for key, entries := range r.PostForm {
		if len(entries) != 1 {
			return nil, fmt.Errorf("duplicate form field")
		}
		values[key] = entries[0]
	}
	return values, nil
}

func contactShapeValid(values map[string]string) bool {
	allowed := map[string]bool{}
	for _, key := range strings.Fields("csrf_token name email type reported_url reason copyrighted_work good_faith accuracy signature request_kind subject message captcha_id captcha_answer captcha captcha_response g-recaptcha-response h-captcha-response website form_token step review_token confirmed security_id component endpoint severity summary steps_to_reproduce impact suggested_fix cve_requested timeline_days credit_preference credit_handle researcher_gpg disclosure_agreement") {
		allowed[key] = true
	}
	for key := range values {
		if !allowed[key] {
			return false
		}
	}
	for _, value := range values {
		if !utf8.ValidString(value) {
			return false
		}
		for _, char := range value {
			if char == '\u200b' || char == '\u200c' || char == '\u200d' || char == '\u202a' || char == '\u202b' || char == '\u202c' || char == '\u202d' || char == '\u202e' || char == '\u2066' || char == '\u2067' || char == '\u2068' || char == '\u2069' {
				return false
			}
		}
	}
	for _, field := range []string{"name", "email", "subject"} {
		if strings.ContainsAny(values[field], "\r\n\x00") {
			return false
		}
	}
	for _, field := range []string{"name", "email", "subject"} {
		for _, char := range values[field] {
			if unicode.IsControl(char) {
				return false
			}
		}
	}
	return !strings.ContainsAny(values["email"], ",;<>")
}

func contactTypeAllowed(kind string, securityMode bool) bool {
	if securityMode {
		return kind == "security"
	}
	return kind == "abuse" || kind == "dmca" || kind == "privacy"
}

func validateContactFields(values map[string]string, kind string) error {
	limits := map[string]int{"name": 100, "email": 254, "subject": 200, "message": 5000, "reported_url": 2048, "reason": 0, "copyrighted_work": 2000, "signature": 100, "request_kind": 0, "component": 255, "endpoint": 2048, "severity": 0, "summary": 200, "steps_to_reproduce": 10000, "impact": 5000, "suggested_fix": 5000, "credit_handle": 100, "researcher_gpg": 16384}
	required := []string{"name", "email"}
	if kind == "security" {
		required = append(required, "component", "severity", "summary", "steps_to_reproduce", "impact", "credit_preference", "disclosure_agreement")
	} else {
		required = append(required, "subject", "message")
	}
	switch kind {
	case "abuse":
		required = append(required, "reported_url", "reason")
	case "dmca":
		required = append(required, "reported_url", "copyrighted_work", "good_faith", "accuracy", "signature")
	case "privacy":
		required = append(required, "request_kind")
	}
	for _, field := range required {
		value := strings.TrimSpace(values[field])
		if value == "" || (field == "good_faith" || field == "accuracy" || field == "disclosure_agreement") && value != "true" {
			return fmt.Errorf("%s is required", field)
		}
	}
	for field, max := range limits {
		if max > 0 && utf8.RuneCountInString(strings.TrimSpace(values[field])) > max {
			return fmt.Errorf("%s exceeds its length limit", field)
		}
	}
	address, err := mail.ParseAddress(strings.TrimSpace(values["email"]))
	if err != nil || !strings.EqualFold(address.Address, strings.TrimSpace(values["email"])) {
		return fmt.Errorf("email must be one valid address")
	}
	if kind == "abuse" && !oneOf(values["reason"], "illegal_content", "terms_violation", "harassment", "malware_phishing", "other") {
		return fmt.Errorf("invalid reason")
	}
	if kind == "privacy" && !oneOf(values["request_kind"], "access", "export", "deletion", "correction", "opt_out", "other") {
		return fmt.Errorf("invalid request kind")
	}
	if kind == "security" {
		if !oneOf(values["severity"], "critical", "high", "medium", "low", "informational") || !oneOf(values["credit_preference"], "name", "handle", "no", "anon") {
			return fmt.Errorf("invalid security report selection")
		}
		if values["credit_preference"] == "handle" && strings.TrimSpace(values["credit_handle"]) == "" {
			return fmt.Errorf("credit handle is required")
		}
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, item := range allowed {
		if value == item {
			return true
		}
	}
	return false
}

func contactBrowserPost(r *http.Request) bool {
	return !strings.Contains(r.URL.Path, "/api/") && !strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "application/json")
}

func contactOriginValid(r *http.Request) bool {
	if site := strings.TrimSpace(r.Header.Get("Sec-Fetch-Site")); site != "" && site != "same-origin" {
		return false
	}
	value := strings.TrimSpace(r.Header.Get("Origin"))
	if value == "" {
		value = strings.TrimSpace(r.Header.Get("Referer"))
	}
	if value == "" {
		return false
	}
	got, err := url.Parse(value)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(strings.TrimSpace(r.URL.Scheme))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return strings.EqualFold(got.Scheme, scheme) && strings.EqualFold(got.Host, r.Host)
}

func contactCaptchaValid(s *Server, r *http.Request, values map[string]string) bool {
	provider := s.contactCaptchaProvider(r)
	if provider == "" {
		provider = "simple"
	}
	response := values["captcha_response"]
	if response == "" {
		if provider == "recaptcha" {
			response = values["g-recaptcha-response"]
		}
		if provider == "hcaptcha" {
			response = values["h-captcha-response"]
		}
	}
	if provider != "simple" && response != "" {
		verified, err := verifyExternalContactCaptcha(r, provider, s.config.Server.Pages.Contact.CaptchaSecretKey, response)
		return err == nil && verified
	}
	return s.consumeContactCaptcha(values["captcha_id"], firstNonEmpty(values["captcha_answer"], values["captcha"]))
}

func validateContactContent(s *Server, r *http.Request, values map[string]string, kind string) error {
	_, err := validateContactContentReason(s, r, values, kind, contactAntispamConfig(s))
	return err
}

func validateContactContentReason(s *Server, r *http.Request, values map[string]string, kind string, settings config.ContactAntispamConfig) (string, error) {
	if kind == "security" {
		return "", nil
	}
	userFields := []string{"name", "email", "reported_url", "reason", "copyrighted_work", "good_faith", "accuracy", "signature", "request_kind", "subject", "message"}
	for _, field := range userFields {
		text := values[field]
		if settings.RejectMarkup && contactMarkupPattern.MatchString(text) {
			return "markup", fmt.Errorf("markup is not allowed")
		}
	}
	for _, word := range settings.BlockedWords {
		word = strings.TrimSpace(word)
		if word == "" {
			continue
		}
		for _, field := range userFields {
			if strings.Contains(strings.ToLower(values[field]), strings.ToLower(word)) {
				return "content", fmt.Errorf("blocked content")
			}
		}
	}
	if _, addressErr := mail.ParseAddress(values["email"]); addressErr == nil {
		address, _ := mail.ParseAddress(values["email"])
		if at := strings.LastIndex(address.Address, "@"); at >= 0 {
			domain := strings.ToLower(strings.TrimSuffix(address.Address[at+1:], "."))
			for _, blocked := range settings.BlockedEmailDomains {
				blocked = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(blocked, "@")))
				if domain == blocked || strings.HasSuffix(domain, "."+blocked) {
					return "content", fmt.Errorf("blocked email domain")
				}
			}
		}
	}
	for _, field := range []string{"name", "subject"} {
		if contactContainsLinks(values[field]) {
			return "content", fmt.Errorf("links are not allowed")
		}
	}
	linkCount := 0
	for _, field := range []string{"message", "copyrighted_work", "steps_to_reproduce", "impact", "suggested_fix"} {
		text := values[field]
		for _, match := range contactLinkPattern.FindAllStringIndex(text, -1) {
			if match[0] > 0 && text[match[0]-1] == '@' {
				continue
			}
			linkCount++
			if settings.MaxLinks >= 0 && linkCount > settings.MaxLinks {
				return "content", fmt.Errorf("too many links")
			}
			link := text[match[0]:match[1]]
			candidate := link
			if !strings.Contains(candidate, "://") {
				candidate = "https://" + candidate
			}
			parsed, err := url.Parse(candidate)
			if err != nil || !contactAllowedHost(r, parsed.Hostname()) {
				return "content", fmt.Errorf("external links are not allowed")
			}
		}
	}
	if values["reported_url"] != "" {
		reported, err := url.ParseRequestURI(values["reported_url"])
		if err != nil || (strings.ToLower(reported.Scheme) != "http" && strings.ToLower(reported.Scheme) != "https") || !contactAllowedHost(r, reported.Hostname()) {
			return "content", fmt.Errorf("reported URL is not allowed")
		}
		if reported.Path == "/" || strings.HasPrefix(reported.Path, "/api/") || strings.HasPrefix(reported.Path, "/server/contact") || s.router == nil {
			return "content", fmt.Errorf("reported URL is not a reportable page")
		}
		ctx := chi.NewRouteContext()
		if !s.router.Match(ctx, http.MethodGet, reported.Path) {
			return "content", fmt.Errorf("reported URL does not exist")
		}
		ident := ""
		for index, key := range ctx.URLParams.Keys {
			if key == "ident" && index < len(ctx.URLParams.Values) {
				ident = ctx.URLParams.Values[index]
				break
			}
		}
		if ident != "" {
			ident = strings.TrimSuffix(ident, ".txt")
			if s.airports == nil {
				return "content", fmt.Errorf("reported resource is unavailable")
			}
			if _, err := s.airports.GetByCode(ident); err != nil {
				return "content", fmt.Errorf("reported resource does not exist")
			}
		}
	}
	return "", nil
}

func contactContainsLinks(value string) bool {
	for _, match := range contactLinkPattern.FindAllStringIndex(value, -1) {
		if match[0] == 0 || value[match[0]-1] != '@' {
			return true
		}
	}
	return false
}

func contactAllowedHost(r *http.Request, host string) bool {
	if strings.EqualFold(host, r.Host) {
		return true
	}
	if base, err := url.Parse(RequestBaseURL(r)); err == nil && strings.EqualFold(host, base.Hostname()) {
		return true
	}
	for _, domain := range strings.Split(os.Getenv("DOMAIN"), ",") {
		if strings.EqualFold(strings.TrimSpace(domain), host) {
			return true
		}
	}
	return false
}

func contactPlainTextBody(values map[string]string, kind string) string {
	var body strings.Builder
	fmt.Fprintf(&body, "Type: %s\n", kind)
	for _, key := range []string{"reported_url", "reason", "copyrighted_work", "good_faith", "accuracy", "signature", "request_kind", "subject", "message"} {
		if value := strings.TrimSpace(values[key]); value != "" {
			fmt.Fprintf(&body, "%s: %s\n", strings.ReplaceAll(key, "_", " "), value)
		}
	}
	return body.String()
}

func contactSuccessMessage(s *Server) string {
	if message := strings.TrimSpace(s.config.Server.Pages.Contact.SuccessMessage); message != "" {
		return message
	}
	return "Your report was received."
}

func (s *Server) renderContactStep(w http.ResponseWriter, r *http.Request, values map[string]string, step, reviewToken, message string) {
	kind := values["type"]
	securityID := values["security_id"]
	data := map[string]interface{}{"Title": "Contact", "Theme": s.config.Web.UI.Theme}
	if step != "success" {
		var err error
		data, err = s.contactPageData(r, kind, kind == "security", securityID, false)
		if err != nil {
			s.writeErrorJSON(w, r, http.StatusServiceUnavailable, "CONTACT_UNAVAILABLE", "The contact form is unavailable.")
			return
		}
	}
	data["ReviewMode"] = step == "review"
	data["SuccessMode"] = step == "success"
	data["ReviewToken"] = reviewToken
	data["StepMessage"] = message
	data["ReviewEmail"] = values["email"]
	data["ReviewType"] = kind
	data["ReviewSubject"] = firstNonEmpty(values["subject"], values["summary"])
	data["ReviewValues"] = values
	fields, _ := data["FormFields"].([]map[string]interface{})
	for _, field := range fields {
		if name, ok := field["name"].(string); ok {
			field["value"] = values[name]
		}
	}
	if contactBrowserPost(r) {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
		s.renderTemplate(w, r, "server_contact", data)
		return
	}
	if step == "review" {
		s.respondItem(w, http.StatusOK, map[string]interface{}{"review": map[string]string{"type": kind, "email": values["email"], "subject": values["subject"]}, "review_token": reviewToken, "confirm_after_seconds": contactAntispamConfig(s).ReviewMinSeconds})
		return
	}
	s.respondItem(w, http.StatusOK, map[string]string{"message": message})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *Server) contactCaptchaProvider(r *http.Request) string {
	if s.isTorRequest(r) || s.isI2PRequest(r) {
		return "simple"
	}
	return strings.ToLower(strings.TrimSpace(s.config.Server.Pages.Contact.Captcha))
}

func verifyExternalContactCaptcha(r *http.Request, provider, secret, response string) (verified bool, resultErr error) {
	endpoint := "https://www.google.com/recaptcha/api/siteverify"
	form := url.Values{"secret": {secret}, "response": {response}}
	if provider == "hcaptcha" {
		endpoint = "https://api.hcaptcha.com/siteverify"
	} else if provider != "recaptcha" {
		return false, fmt.Errorf("unsupported CAPTCHA provider")
	}
	if response == "" || secret == "" {
		return false, nil
	}
	if provider == "recaptcha" {
		if remoteIP := middleware.GetClientIP(r.Context()); remoteIP != "" {
			form.Set("remoteip", remoteIP)
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false, fmt.Errorf("create CAPTCHA verification request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("verify CAPTCHA: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close CAPTCHA provider response: %w", err))
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("CAPTCHA provider returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&result); err != nil {
		return false, fmt.Errorf("decode CAPTCHA provider response: %w", err)
	}
	return result.Success, nil
}

func (s *Server) submitSecurityReport(w http.ResponseWriter, r *http.Request, name, email string) error {
	component := strings.TrimSpace(r.FormValue("component"))
	severity := strings.ToLower(strings.TrimSpace(r.FormValue("severity")))
	summary := strings.TrimSpace(r.FormValue("summary"))
	steps := strings.TrimSpace(r.FormValue("steps_to_reproduce"))
	impact := strings.TrimSpace(r.FormValue("impact"))
	credit := strings.TrimSpace(r.FormValue("credit_preference"))
	creditName := strings.TrimSpace(r.FormValue("credit_handle"))
	endpoint := strings.TrimSpace(r.FormValue("endpoint"))
	timeline := strings.TrimSpace(r.FormValue("timeline_days"))
	if component == "" || utf8.RuneCountInString(component) > 255 || summary == "" || utf8.RuneCountInString(summary) > 200 || steps == "" || utf8.RuneCountInString(steps) > 10000 || impact == "" || utf8.RuneCountInString(impact) > 5000 {
		return fmt.Errorf("complete the required security report fields within their length limits")
	}
	switch severity {
	case "critical", "high", "medium", "low", "informational":
	default:
		return fmt.Errorf("select a valid severity")
	}
	switch credit {
	case "name":
		credit = "name"
		creditName = name
	case "handle":
		if creditName == "" {
			return fmt.Errorf("credit handle is required")
		}
	case "no":
	case "anon":
	default:
		return fmt.Errorf("select a credit preference")
	}
	if r.FormValue("disclosure_agreement") != "true" {
		return fmt.Errorf("coordinated disclosure agreement is required")
	}
	if timeline == "" {
		timeline = "90"
	}
	days, err := strconv.Atoi(timeline)
	if err != nil || days < 1 || days > 365 {
		return fmt.Errorf("timeline must be a number of days")
	}
	if researcherKey := strings.TrimSpace(r.FormValue("researcher_gpg")); strings.HasPrefix(strings.ToLower(researcherKey), "https://") && utf8.RuneCountInString(researcherKey) > 2048 {
		return fmt.Errorf("researcher GPG URL is too long")
	}
	securityID := strings.TrimSpace(r.FormValue("security_id"))
	report := map[string]interface{}{
		"security_id": securityID, "component": component, "endpoint": endpoint,
		"severity": severity, "summary": summary, "steps_to_reproduce": steps,
		"impact": impact, "suggested_fix": strings.TrimSpace(r.FormValue("suggested_fix")),
		"cve_requested": r.FormValue("cve_requested") == "true", "timeline_days": timeline,
		"credit_preference": credit,
		"app_version":       Version, "commit_hash": CommitID, "timestamp": time.Now().UTC().Format(time.RFC3339),
		"request_user_agent": r.UserAgent(),
	}
	plain, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("could not prepare the security report")
	}
	SecurityReportMu.Lock()
	locked := true
	defer func() {
		if locked {
			SecurityReportMu.Unlock()
		}
	}()
	bodyAlgorithm := "aes-256-gcm"
	var ciphertext []byte
	publicKey, keyErr := s.pgpPublicKey()
	if keyErr == nil {
		ciphertext, err = pgp.EncryptTo(publicKey, plain)
		bodyAlgorithm = "pgp"
	} else if errors.Is(keyErr, os.ErrNotExist) {
		ciphertext, err = s.encryptSecurityData(plain)
	} else {
		err = keyErr
	}
	if err != nil {
		return fmt.Errorf("security report encryption is unavailable")
	}
	contactPlain, err := json.Marshal(map[string]string{
		"name": name, "email": email, "researcher_gpg": strings.TrimSpace(r.FormValue("researcher_gpg")),
		"credit_name": creditName,
	})
	if err != nil {
		return fmt.Errorf("could not prepare the security report contact data")
	}
	contactCiphertext, err := s.encryptSecurityData(contactPlain)
	if err != nil {
		return fmt.Errorf("security report encryption is unavailable")
	}
	if err := s.ensureDisclosureSchema(); err != nil {
		return fmt.Errorf("security report storage is unavailable")
	}
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return fmt.Errorf("could not allocate a report tracking id")
	}
	trackingID := "sec_" + hex.EncodeToString(idBytes)
	statusTokenBytes := make([]byte, 32)
	if _, err := rand.Read(statusTokenBytes); err != nil {
		return fmt.Errorf("could not allocate a report status token")
	}
	statusToken := hex.EncodeToString(statusTokenBytes)
	statusTokenHash := sha256.Sum256([]byte(statusToken))
	created := time.Now().UTC().Format(time.RFC3339)
	creditNameValue := any(creditName)
	if credit == "anon" || credit == "no" {
		creditNameValue = nil
	}
	_, err = s.db.Exec(`INSERT INTO security_reports(
		tracking_id, security_id, severity, component, status, body_enc, body_alg, contact_enc,
		credit_pref, credit_name, cve_requested, token_hash, created_at, updated_at
	) VALUES(?, ?, ?, ?, 'received', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		trackingID, securityID, severity, component, ciphertext, bodyAlgorithm, contactCiphertext, credit, creditNameValue,
		r.FormValue("cve_requested") == "true", hex.EncodeToString(statusTokenHash[:]), created, created)
	if err != nil {
		return fmt.Errorf("security report storage is unavailable")
	}
	SecurityReportMu.Unlock()
	locked = false
	if s.securityLog != nil {
		line := fmt.Sprintf("security.report_received tracking_id=%s severity=%s component=%q", trackingID, severity, component)
		if err := s.securityLog.Write(line); err != nil {
			slog.Error("security log write failed", "tracking_id", trackingID, "error", err)
		}
	}
	if notify.CanSend(s.config) {
		if recipient := s.config.Server.Contact.ResolveEmail("security"); recipient != "" {
			if err := notify.SendEncryptedSecurityReport(s.config, recipient, bodyAlgorithm, ciphertext); err != nil {
				slog.Error("security report notification failed", "tracking_id", trackingID, "error", err)
			}
		}
	}
	securityWebhookTargets := s.contactWebhookTargets("security")
	if len(securityWebhookTargets) > 0 {
		message := notify.WebhookMessage{
			Event: "security.report_received", Subject: "[" + s.config.Server.Branding.Title + " security] Report received",
			Body:     "Encrypted security report (" + bodyAlgorithm + "): " + base64.StdEncoding.EncodeToString(ciphertext),
			Severity: severity, Timestamp: time.Now().UTC(), ProjectName: s.config.Server.Branding.Title,
			ProjectVersion: Version, AppURL: strings.TrimSuffix(RequestBaseURL(r), "/"), TrackingID: trackingID, Component: "security",
		}
		s.dispatchContactWebhooks(r, securityWebhookTargets, message)
	}
	adminWebhookTargets := s.contactWebhookTargetsForRole("admin", s.config.Server.Contact.Admin)
	if len(adminWebhookTargets) > 0 {
		message := notify.WebhookMessage{
			Event: "security.report_received", Subject: "[" + s.config.Server.Branding.Title + " security] Report received",
			Body:     "A security report was received. Tracking ID: " + trackingID + "\nSeverity: " + severity,
			Severity: severity, Timestamp: time.Now().UTC(), ProjectName: s.config.Server.Branding.Title,
			ProjectVersion: Version, AppURL: strings.TrimSuffix(RequestBaseURL(r), "/"), TrackingID: trackingID, Component: "security",
		}
		s.dispatchContactWebhooks(r, adminWebhookTargets, message)
	}
	acknowledgmentSent := false
	if notify.CanSend(s.config) {
		if err := notify.Send(s.config, s.configDir, "security_report_acknowledgment", email, map[string]string{
			"tracking_id": trackingID,
			"status_url":  strings.TrimSuffix(RequestBaseURL(r), "/") + "/server/security/report/" + trackingID + "?token=" + statusToken,
		}); err == nil {
			acknowledgmentSent = true
		} else {
			slog.Error("security report acknowledgment failed", "tracking_id", trackingID, "error", err)
		}
	}
	message := "We received your report. Your tracking ID is " + trackingID + "."
	if acknowledgmentSent {
		message += " A confirmation email was sent."
	}
	if contactBrowserPost(r) {
		values := make(map[string]string)
		for key := range r.Form {
			values[key] = r.Form.Get(key)
		}
		s.renderContactStep(w, r, values, "success", "", message)
		return nil
	}
	s.respondItem(w, http.StatusOK, map[string]string{"tracking_id": trackingID, "message": message})
	return nil
}
