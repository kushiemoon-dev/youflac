package backend

import (
	"log/slog"
	"testing"
)

func TestInitLogger_Info(t *testing.T) {
	InitLogger("info")
	if slog.Default() == nil {
		t.Error("expected non-nil logger after InitLogger")
	}
}

func TestInitLogger_Debug(t *testing.T) {
	InitLogger("debug")
}

func TestInitLogger_Warn(t *testing.T) {
	InitLogger("warn")
}

func TestInitLogger_Error(t *testing.T) {
	InitLogger("error")
}

func TestInitLogger_Unknown(t *testing.T) {
	// Unknown level should default to info without panicking
	InitLogger("unknown_level")
}

func TestInitLogger_Empty(t *testing.T) {
	InitLogger("")
}
