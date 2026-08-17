// Package version provides build-time version information and the single
// identity string used by every OTel tracer/meter in the module.
// Version and BuildHash are populated via ldflags during the build process.
package version

// ServiceName names the instrumentation scope of every tracer and meter in this
// module. It is the compile-time identity — the runtime service.name reported to
// OTel comes from config (server.name / OTEL_SERVICE_NAME) and may differ per
// deployment. Rename this once when forking the template.
const ServiceName = "goth-template"

// Version is the semantic version of the application.
// Set at build time via: -X github.com/effiware/goth-template/internal/version.Version=x.y.z
var Version = "0.0.0"

// BuildHash is the git commit hash of the build.
// Set at build time via: -X github.com/effiware/goth-template/internal/version.BuildHash=<commit-sha>
var BuildHash = "unknown"
