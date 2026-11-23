// Package compliance resolves the explicit requirements matrix in AI.md PART 11.
// It describes requirements; it does not certify that the application meets them.
package compliance

import "github.com/apimgr/airports/src/config"

// EffectiveRequirements contains the matrix requirements that have a defined
// strictest-value rule. A zero value means the enabled standards define no
// value for that field, not that the requirement is satisfied.
type EffectiveRequirements struct {
	Standards               []string
	AuditRetentionYears     int
	EncryptAtRest           bool
	EncryptInTransit        bool
	RightToErasure          bool
	DataPortability         bool
	ConsentTracking         bool
	AccessLogging           bool
	BreachNotification      bool
	BreachNotificationHours int
	DataResidency           bool
	StrongAuthentication    bool
	TokenPolicy             bool
	SessionTimeoutRequired  bool
	VulnerabilityScanning   bool
}

// Resolve applies PART 11's strictest-wins rule to all configured standards.
// Matrix fields are populated from the matrix and standard-specific behaviors;
// controls without a corresponding field are not inferred. Numeric breach
// deadlines are reported only where PART 11 specifies one.
func Resolve(enabled config.ComplianceConfig) EffectiveRequirements {
	result := EffectiveRequirements{}
	add := func(name string, retention int, atRest, inTransit, erase, portability, consent, access, breach bool, breachHours int, residency, strongAuth, token, session, scanning bool) {
		result.Standards = append(result.Standards, name)
		if retention > result.AuditRetentionYears {
			result.AuditRetentionYears = retention
		}
		result.EncryptAtRest = result.EncryptAtRest || atRest
		result.EncryptInTransit = result.EncryptInTransit || inTransit
		result.RightToErasure = result.RightToErasure || erase
		result.DataPortability = result.DataPortability || portability
		result.ConsentTracking = result.ConsentTracking || consent
		result.AccessLogging = result.AccessLogging || access
		result.BreachNotification = result.BreachNotification || breach
		if breachHours > 0 && (result.BreachNotificationHours == 0 || breachHours < result.BreachNotificationHours) {
			result.BreachNotificationHours = breachHours
		}
		result.DataResidency = result.DataResidency || residency
		result.StrongAuthentication = result.StrongAuthentication || strongAuth
		result.TokenPolicy = result.TokenPolicy || token
		result.SessionTimeoutRequired = result.SessionTimeoutRequired || session
		result.VulnerabilityScanning = result.VulnerabilityScanning || scanning
	}
	if enabled.GDPR {
		add("gdpr", 1, true, true, true, true, true, true, true, 72, true, false, true, false, false)
	}
	if enabled.CCPA {
		add("ccpa", 1, false, false, true, true, true, false, false, 0, false, false, false, false, false)
	}
	if enabled.HIPAA {
		add("hipaa", 6, true, true, false, false, true, true, true, 24*60, false, true, true, false, false)
	}
	if enabled.SOC2 {
		add("soc2", 1, true, true, false, false, false, true, true, 0, false, true, true, false, true)
	}
	if enabled.PCIDSS {
		add("pci_dss", 1, true, true, false, false, false, true, true, 0, false, true, true, false, true)
	}
	if enabled.ISO27001 {
		add("iso27001", 3, true, true, false, false, false, true, true, 0, false, false, true, false, true)
	}
	if enabled.FedRAMP {
		add("fedramp", 0, false, false, false, false, false, false, true, 0, true, false, false, false, true)
	}
	if enabled.LGPD {
		add("lgpd", 0, false, false, true, true, true, false, true, 0, false, false, false, false, false)
	}
	if enabled.PIPEDA {
		add("pipeda", 0, false, false, false, false, true, false, true, 0, false, false, false, false, false)
	}
	if enabled.APPI {
		add("appi", 0, false, false, false, false, true, false, true, 0, false, false, false, false, false)
	}
	if enabled.PDPA {
		add("pdpa", 0, false, false, false, false, true, false, true, 72, false, false, false, false, false)
	}
	return result
}
