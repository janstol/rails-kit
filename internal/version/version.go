package version

import "runtime/debug"

var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

// Info returns version metadata, preferring ldflags and falling back to Go
// build info when the binary was installed with go install.
func Info() (string, string, string) {
	version := Version
	commit := Commit
	buildDate := BuildDate

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return version, commit, buildDate
	}

	if version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}

	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}

	if commit == "none" {
		if revision := settings["vcs.revision"]; revision != "" {
			commit = revision
			if len(commit) > 7 {
				commit = commit[:7]
			}
		}
	}

	if buildDate == "unknown" {
		if vcsTime := settings["vcs.time"]; vcsTime != "" {
			buildDate = vcsTime
		}
	}

	return version, commit, buildDate
}
