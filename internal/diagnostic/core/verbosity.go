package core

import (
	"fmt"
	"os"
	"sync"
)

// VerbosityLevel defines the detail level for output
type VerbosityLevel int

const (
	// NormalVerbosity is the standard output level
	NormalVerbosity VerbosityLevel = iota

	// DetailedVerbosity shows more detailed information
	DetailedVerbosity

	// DebugVerbosity shows all available information including debug messages
	DebugVerbosity
)

// Global verbosity state
var (
	globalVerbosityLevel VerbosityLevel = NormalVerbosity
	globalVerbose        bool           = false
	verbosityMutex       sync.RWMutex
)

// SetVerbosity sets the global verbosity level
func SetVerbosity(verbose bool) {
	verbosityMutex.Lock()
	defer verbosityMutex.Unlock()

	globalVerbose = verbose

	if verbose {
		globalVerbosityLevel = DetailedVerbosity
		fmt.Fprintf(os.Stderr, "Verbose mode enabled - detailed output will be shown\n")
	} else {
		globalVerbosityLevel = NormalVerbosity
	}
}

// IsVerbose returns whether verbose mode is enabled
func IsVerbose() bool {
	verbosityMutex.RLock()
	defer verbosityMutex.RUnlock()
	return globalVerbose
}

// GetVerbosityLevel returns the current verbosity level
func GetVerbosityLevel() VerbosityLevel {
	verbosityMutex.RLock()
	defer verbosityMutex.RUnlock()
	return globalVerbosityLevel
}

// VerbosePrintf prints formatted output only when verbose mode is enabled
func VerbosePrintf(format string, args ...interface{}) {
	if IsVerbose() {
		fmt.Printf(format, args...)
	}
}

// VerbosePrintln prints a line only when verbose mode is enabled
func VerbosePrintln(args ...interface{}) {
	if IsVerbose() {
		fmt.Println(args...)
	}
}

// VerboseSection prints a section header only when verbose mode is enabled
func VerboseSection(title string) {
	if IsVerbose() {
		fmt.Printf("\n=== %s ===\n", title)
	}
}

// VerboseDetail prints detailed information only when verbose mode is enabled
func VerboseDetail(format string, args ...interface{}) {
	if IsVerbose() {
		fmt.Printf("  - %s\n", fmt.Sprintf(format, args...))
	}
}
