//go:build !eiksy_debug

package debuglog

func Path() string { return "" }

func Printf(string, ...interface{}) {}

func RecoverPanic(string) {}
