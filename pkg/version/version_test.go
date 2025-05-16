package version

import (
	"fmt"
	"log/slog"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrint(t *testing.T) {
	goVersion := runtime.Version()

	Version = "v0.7.0-29-gcc373f2"
	Revision = "cc373f263575773f1349bbd354e803cc85f9edcd"
	Branch = "main"
	BuildUser = "root"
	BuildDate = "2021-12-23@09:46:17"

	version, err := Print("klogs")
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("klogs, version v0.7.0-29-gcc373f2 (branch: main, revision: cc373f263575773f1349bbd354e803cc85f9edcd)\n  build user:       root\n  build date:       2021-12-23@09:46:17\n  go version:       %s", goVersion), version)
}

func TestInfo(t *testing.T) {
	Version = "v0.7.0-29-gcc373f2"
	Revision = "cc373f263575773f1349bbd354e803cc85f9edcd"
	Branch = "main"

	fields := Info()
	require.Equal(t, []slog.Attr{slog.String("version", "v0.7.0-29-gcc373f2"), slog.String("branch", "main"), slog.String("revision", "cc373f263575773f1349bbd354e803cc85f9edcd")}, fields)
}

func TestBuildContext(t *testing.T) {
	goVersion := runtime.Version()

	BuildUser = "root"
	BuildDate = "2021-12-23@09:46:17"

	fields := BuildContext()
	require.Equal(t, []slog.Attr{slog.String("go", goVersion), slog.String("user", "root"), slog.String("date", "2021-12-23@09:46:17")}, fields)
}
