package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/mail"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/airports/src/notify"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	contactCaptchaLifetime = 10 * time.Minute
	contactFailureLifetime = 15 * time.Minute
	contactCaptchaLimit    = 4096
	contactFormMaxBytes    = 32 << 10
)

type contactCaptchaChallenge struct {
	answer  int
	expires time.Time
}

func (s *Server) newContactCaptcha() (string, string, error) {
	left, err := rand.Int(rand.Reader, big.NewInt(9))
	if err != nil {
		return "", "", err
	}
	right, err := rand.Int(rand.Reader, big.NewInt(9))
	if err != nil {
		return "", "", err
	}
	challengeIDBytes := make([]byte, 24)
	if _, err := rand.Read(challengeIDBytes); err != nil {
		return "", "", err
	}
	challengeID := hex.EncodeToString(challengeIDBytes)
	a, b := int(left.Int64())+1, int(right.Int64())+1
	now := time.Now()
	s.contactCaptchaMu.Lock()
	defer s.contactCaptchaMu.Unlock()
	for id, challenge := range s.contactCaptchaChallenges {
		if !now.Before(challenge.expires) {
			delete(s.contactCaptchaChallenges, id)
		}
	}
	if len(s.contactCaptchaChallenges) >= contactCaptchaLimit {
		return "", "", fmt.Errorf("contact CAPTCHA challenge limit reached")
	}
	s.contactCaptchaChallenges[challengeID] = contactCaptchaChallenge{answer: a + b, expires: now.Add(contactCaptchaLifetime)}
	return fmt.Sprintf("What is %d + %d?", a, b), challengeID, nil
}

func (s *Server) consumeContactCaptcha(challengeID, submitted string) bool {
	s.contactCaptchaMu.Lock()
	challenge, ok := s.contactCaptchaChallenges[challengeID]
	delete(s.contactCaptchaChallenges, challengeID)
	s.contactCaptchaMu.Unlock()
	if !ok || !time.Now().Before(challenge.expires) {
		return false
	}
	answer, err := strconv.Atoi(strings.TrimSpace(submitted))
	return err == nil && answer == challenge.answer
}

func contactCaptchaIdentity(r *http.Request) string {
	identity := ""
	if circuitID := TorCircuitID(r); circuitID != "" {
		identity = "tor:" + circuitID
	} else {
		identity = middleware.GetClientIP(r.Context())
		if identity == "" {
			identity = r.RemoteAddr
		}
		if host, _, err := net.SplitHostPort(identity); err == nil {
			identity = host
		}
	}
	digest := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(digest[:])
}

func (s *Server) contactCaptchaIsRequired(r *http.Request) bool {
	key := contactCaptchaIdentity(r)
	now := time.Now()
	s.contactCaptchaMu.Lock()
	defer s.contactCaptchaMu.Unlock()
	for identity, expiry := range s.contactCaptchaRequired {
		if !now.Before(expiry) {
			delete(s.contactCaptchaRequired, identity)
		}
	}
	expiry, ok := s.contactCaptchaRequired[key]
	return ok && now.Before(expiry)
}

func (s *Server) markContactCaptchaRequired(r *http.Request) {
	now := time.Now()
	key := contactCaptchaIdentity(r)
	s.contactCaptchaMu.Lock()
	defer s.contactCaptchaMu.Unlock()
	for identity, expiry := range s.contactCaptchaRequired {
		if !now.Before(expiry) {
			delete(s.contactCaptchaRequired, identity)
		}
	}
	if len(s.contactCaptchaRequired) < contactCaptchaLimit || s.contactCaptchaRequired[key].IsZero() {
		s.contactCaptchaRequired[key] = now.Add(contactFailureLifetime)
	}
}

func (s *Server) clearContactCaptchaRequired(r *http.Request) {
	s.contactCaptchaMu.Lock()
	delete(s.contactCaptchaRequired, contactCaptchaIdentity(r))
	s.contactCaptchaMu.Unlock()
}

func (s *Server) handleContactSubmission(w http.ResponseWriter, r *http.Request) {
	if !s.config.Server.Pages.Contact.Enabled {
		http.NotFound(w, r)
		return
	}
	submitted := false
	defer func() {
		if submitted {
			s.clearContactCaptchaRequired(r)
			return
		}
		s.markContactCaptchaRequired(r)
	}()
	captchaType := strings.ToLower(strings.TrimSpace(s.config.Server.Pages.Contact.Captcha))
	if captchaType == "" {
		captchaType = "simple"
	}
	if captchaType != "simple" {
		message := "The configured contact CAPTCHA provider is unavailable."
		if s.isTorRequest(r) {
			message = "The configured CAPTCHA provider is not available over Tor; choose the self-hosted simple challenge."
		}
		s.writeErrorJSON(w, r, http.StatusServiceUnavailable, "CAPTCHA_UNAVAILABLE", message)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, contactFormMaxBytes)
	if err := r.ParseForm(); err != nil {
		s.writeErrorJSON(w, r, http.StatusBadRequest, "INVALID_CONTACT_FORM", "The contact form is invalid or too large.")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	subject := strings.TrimSpace(r.FormValue("subject"))
	message := strings.TrimSpace(r.FormValue("message"))
	if name == "" || len(name) > 200 || email == "" || len(email) > 320 ||
		subject == "" || len(subject) > 200 || strings.ContainsAny(subject, "\r\n") ||
		message == "" || len(message) > 16<<10 {
		s.writeErrorJSON(w, r, http.StatusBadRequest, "INVALID_CONTACT_FORM", "Complete each contact field within its length limit.")
		return
	}
	parsedEmail, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(parsedEmail.Address, email) {
		s.writeErrorJSON(w, r, http.StatusBadRequest, "INVALID_CONTACT_FORM", "Enter a valid email address.")
		return
	}
	if s.contactCaptchaIsRequired(r) && !s.consumeContactCaptcha(r.FormValue("captcha_id"), r.FormValue("captcha")) {
		s.writeErrorJSON(w, r, http.StatusBadRequest, "CAPTCHA_FAILED", "The arithmetic answer was incorrect or expired. Reload the contact page and try again.")
		return
	}
	if !notify.CanSend(s.config) {
		s.writeErrorJSON(w, r, http.StatusServiceUnavailable, "CONTACT_UNAVAILABLE", "Contact submissions are unavailable because email delivery is not configured.")
		return
	}
	recipient := s.config.Server.Contact.ResolveEmail("general")
	if recipient == "" {
		s.writeErrorJSON(w, r, http.StatusServiceUnavailable, "CONTACT_UNAVAILABLE", "No contact recipient is configured.")
		return
	}
	if err := notify.Send(s.config, s.configDir, "contact_submission", recipient, map[string]string{
		"sender_name":     name,
		"sender_email":    email,
		"contact_subject": subject,
		"contact_message": message,
	}); err != nil {
		slog.Error("contact form delivery failed", "request_id", middleware.GetReqID(r.Context()), "error", err)
		s.writeErrorJSON(w, r, http.StatusBadGateway, "CONTACT_DELIVERY_FAILED", "The contact message could not be delivered. Please try again later.")
		return
	}
	success := strings.TrimSpace(s.config.Server.Pages.Contact.SuccessMessage)
	if success == "" {
		success = "Thank you for your message. We'll respond soon."
	}
	submitted = true
	s.respondItem(w, http.StatusOK, map[string]string{"message": success})
}
