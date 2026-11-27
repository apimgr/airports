package logging

import (
	"io"
	"sync"
)

var consoleWriteMu sync.Mutex

type synchronizedWriter struct {
	writer io.Writer
}

func (writer synchronizedWriter) Write(data []byte) (int, error) {
	consoleWriteMu.Lock()
	defer consoleWriteMu.Unlock()
	return writer.writer.Write(data)
}

func NewSynchronizedWriter(writer io.Writer) io.Writer {
	return synchronizedWriter{writer: writer}
}

func WriteSynchronized(writer io.Writer, data []byte) (int, error) {
	consoleWriteMu.Lock()
	defer consoleWriteMu.Unlock()
	return writer.Write(data)
}
