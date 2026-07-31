package cmd

// jsonSchemaVersion is the --json output contract version. Bump only on a
// breaking change (field removed/renamed, type changed, error code's meaning
// changed) — see docs/json.md.
const jsonSchemaVersion = 1

// Closed vocabulary of JSON error codes. Adding a code is additive; changing
// what an existing one means is breaking. Anything not explicitly coded below
// falls through to codeInternal — an honest fallback rather than a fabricated
// taxonomy.
const (
	codeNotARailsRoot   = "not_a_rails_root"
	codeNotFound        = "not_found"
	codeInvalidArgument = "invalid_argument"
	codeParseError      = "parse_error"
	codeInternal        = "internal"
)

// jsonEnvelope wraps every successful --json payload written to stdout.
type jsonEnvelope struct {
	SchemaVersion int    `json:"schema_version"`
	Command       string `json:"command"`
	Data          any    `json:"data"`
}

// jsonErrorEnvelope wraps every failing --json payload written to stderr.
type jsonErrorEnvelope struct {
	SchemaVersion int       `json:"schema_version"`
	Command       string    `json:"command"`
	Error         jsonError `json:"error"`
}

type jsonError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// codedError attaches a closed-vocabulary error code to an existing error so
// the top-level error handler can recover it via errors.As without the
// internal packages knowing anything about JSON.
type codedError struct {
	code string
	err  error
}

func coded(code string, err error) error {
	if err == nil {
		return nil
	}
	return &codedError{code: code, err: err}
}

func (e *codedError) Error() string { return e.err.Error() }
func (e *codedError) Unwrap() error { return e.err }
