package mode

// GetErrorDetail returns detailed errors in development and generic errors in
// production.
func GetErrorDetail(err error) string {
	if err == nil {
		return ""
	}
	if IsAppModeDev() || GetAppMode() == Debug {
		return err.Error()
	}
	return "An internal error occurred. Please contact support if the problem persists."
}

// ShouldEnableAutoReload reports whether development behavior should reload
// templates and static assets automatically.
func ShouldEnableAutoReload() bool {
	return IsAppModeDev() || GetAppMode() == Debug
}

// GetPanicRecoveryMode returns verbose recovery for development and graceful
// recovery for production.
func GetPanicRecoveryMode() string {
	if IsAppModeDev() || GetAppMode() == Debug {
		return "verbose"
	}
	return "graceful"
}
