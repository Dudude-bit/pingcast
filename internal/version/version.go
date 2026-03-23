package version

// Build-time variables injected via -ldflags.
// Example:
//
//	go build -ldflags "-X github.com/kirillinakin/pingcast/internal/version.Version=v1.0.0 \
//	  -X github.com/kirillinakin/pingcast/internal/version.Commit=abc1234 \
//	  -X github.com/kirillinakin/pingcast/internal/version.BuildTime=2026-03-23T00:00:00Z"
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)
