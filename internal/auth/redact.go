package auth

import "log/slog"

// redactedToken renders a token as presence and length in log output, so a
// raw secret value never reaches a log line even through a %v or JSON
// handler.
type redactedToken string

func (t redactedToken) LogValue() slog.Value {
	if t == "" {
		return slog.GroupValue(slog.Bool("present", false))
	}
	return slog.GroupValue(
		slog.Bool("present", true),
		slog.Int("length", len(t)),
	)
}
