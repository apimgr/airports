package signal

import "os"

func shutdownSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

func logReopenSignals() []os.Signal {
	return nil
}

func statusDumpSignals() []os.Signal {
	return nil
}

func ignoreConfigReloadSignal() {}
