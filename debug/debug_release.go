// +build !debug

package debug

func Log(tag string, fmt string, args ...interface{}) {}

func Break(string) {}

func BreakIf(string, func() bool) {}
