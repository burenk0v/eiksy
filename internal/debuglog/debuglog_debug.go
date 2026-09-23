//go:build eiksy_debug

package debuglog

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var mu sync.Mutex

func Path() string {
	return filepath.Join(os.TempDir(), "eiksy", "debug.log")
}

func Printf(format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()

	path := Path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer f.Close()

	_, _ = fmt.Fprintf(f, "%s [%s/%s] %s\\n", time.Now().UTC().Format(time.RFC3339Nano), runtime.GOOS, runtime.GOARCH, fmt.Sprintf(format, args...))
}

func RecoverPanic(stage string) {
	if recovered := recover(); recovered != nil {
		Printf("panic during %s: %v", stage, recovered)
		panic(recovered)
	}
}
