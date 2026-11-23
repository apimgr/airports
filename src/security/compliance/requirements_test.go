package compliance

import (
	"reflect"
	"testing"

	"github.com/apimgr/airports/src/config"
)

func TestResolveAppliesStrictestMatrixRequirements(t *testing.T) {
	got := Resolve(config.ComplianceConfig{GDPR: true, HIPAA: true, ISO27001: true})
	wantStandards := []string{"gdpr", "hipaa", "iso27001"}
	if !reflect.DeepEqual(got.Standards, wantStandards) {
		t.Fatalf("standards = %v, want %v", got.Standards, wantStandards)
	}
	if got.AuditRetentionYears != 6 {
		t.Errorf("audit retention = %d, want 6", got.AuditRetentionYears)
	}
	if got.BreachNotificationHours != 72 {
		t.Errorf("breach deadline = %d hours, want 72", got.BreachNotificationHours)
	}
	if !got.EncryptAtRest || !got.EncryptInTransit || !got.RightToErasure || !got.DataPortability || !got.ConsentTracking || !got.AccessLogging || !got.DataResidency || !got.TokenPolicy || !got.VulnerabilityScanning {
		t.Errorf("missing enabled matrix requirements: %+v", got)
	}
	if !got.StrongAuthentication || got.SessionTimeoutRequired {
		t.Errorf("strong authentication or no-session requirements are incorrect: %+v", got)
	}
}

func TestResolveIncludesAllConfiguredStandards(t *testing.T) {
	got := Resolve(config.ComplianceConfig{
		GDPR: true, CCPA: true, HIPAA: true, SOC2: true, PCIDSS: true,
		ISO27001: true, FedRAMP: true, LGPD: true, PIPEDA: true, APPI: true, PDPA: true,
	})
	wantStandards := []string{"gdpr", "ccpa", "hipaa", "soc2", "pci_dss", "iso27001", "fedramp", "lgpd", "pipeda", "appi", "pdpa"}
	if !reflect.DeepEqual(got.Standards, wantStandards) {
		t.Fatalf("standards = %v, want %v", got.Standards, wantStandards)
	}
	if got.AuditRetentionYears != 6 || got.BreachNotificationHours != 72 {
		t.Errorf("strictest defined values = retention %d years, breach %d hours; want 6 years and 72 hours", got.AuditRetentionYears, got.BreachNotificationHours)
	}
	if !got.DataResidency || !got.VulnerabilityScanning || !got.RightToErasure || !got.DataPortability || !got.ConsentTracking || !got.BreachNotification {
		t.Errorf("missing standard-specific requirements: %+v", got)
	}
	if got.SessionTimeoutRequired {
		t.Errorf("session timeout requirement set despite the project having no sessions: %+v", got)
	}
}

func TestResolveIncludesEachAdditionalStandard(t *testing.T) {
	tests := []struct {
		name     string
		config   config.ComplianceConfig
		standard string
		check    func(EffectiveRequirements) bool
	}{
		{"FedRAMP", config.ComplianceConfig{FedRAMP: true}, "fedramp", func(g EffectiveRequirements) bool {
			return g.DataResidency && g.VulnerabilityScanning && g.BreachNotification
		}},
		{"LGPD", config.ComplianceConfig{LGPD: true}, "lgpd", func(g EffectiveRequirements) bool {
			return g.RightToErasure && g.DataPortability && g.ConsentTracking && g.BreachNotification
		}},
		{"PIPEDA", config.ComplianceConfig{PIPEDA: true}, "pipeda", func(g EffectiveRequirements) bool { return g.ConsentTracking && g.BreachNotification }},
		{"APPI", config.ComplianceConfig{APPI: true}, "appi", func(g EffectiveRequirements) bool { return g.ConsentTracking && g.BreachNotification }},
		{"PDPA", config.ComplianceConfig{PDPA: true}, "pdpa", func(g EffectiveRequirements) bool {
			return g.ConsentTracking && g.BreachNotification && g.BreachNotificationHours == 72
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Resolve(tt.config)
			if !reflect.DeepEqual(got.Standards, []string{tt.standard}) {
				t.Fatalf("standards = %v, want [%s]", got.Standards, tt.standard)
			}
			if !tt.check(got) {
				t.Errorf("missing mapped requirements: %+v", got)
			}
		})
	}
}

func TestResolveNoStandardsIsEmpty(t *testing.T) {
	got := Resolve(config.ComplianceConfig{})
	if len(got.Standards) != 0 || got.AuditRetentionYears != 0 || got.BreachNotificationHours != 0 {
		t.Fatalf("empty configuration resolved to requirements: %+v", got)
	}
}
