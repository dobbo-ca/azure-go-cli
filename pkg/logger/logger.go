package logger

import (
  "fmt"
  "io"
  "os"

  "github.com/Azure/azure-sdk-for-go/sdk/azcore/log"
)

var (
  // DebugEnabled controls whether debug messages are printed
  DebugEnabled = false
  // Output is the writer for debug messages (defaults to stderr)
  Output io.Writer = os.Stderr
)

// EnableDebug enables debug logging for both our logger and Azure SDK
func EnableDebug() {
  DebugEnabled = true

  // Enable Azure SDK logging
  log.SetListener(func(event log.Event, message string) {
    fmt.Fprintf(Output, "[AZURE-SDK:%s] %s\n", event, message)
  })

  // Set which Azure SDK events to log
  log.SetEvents(
    log.EventRequest,
    log.EventResponse,
    log.EventRetryPolicy,
  )
}

// DisableDebug disables debug logging
func DisableDebug() {
  DebugEnabled = false
  log.SetListener(nil)
}

// Debug prints a debug message if debug mode is enabled
func Debug(format string, args ...interface{}) {
  if DebugEnabled {
    fmt.Fprintf(Output, "[DEBUG] "+format+"\n", args...)
  }
}

// Info prints an informational message
func Info(format string, args ...interface{}) {
  fmt.Fprintf(Output, "[INFO] "+format+"\n", args...)
}

// Error prints an error message
func Error(format string, args ...interface{}) {
  fmt.Fprintf(Output, "[ERROR] "+format+"\n", args...)
}
