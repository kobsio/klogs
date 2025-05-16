package version

import (
	"bytes"
	"log/slog"
	"runtime"
	"strings"
	"text/template"
)

// Build information. Populated at build-time.
var (
	Version   string
	Revision  string
	Branch    string
	BuildUser string
	BuildDate string
	GoVersion = runtime.Version()
)

// versionInfoTmpl contains the template used by Print.
var versionInfoTmpl = `
{{.program}}, version {{.version}} (branch: {{.branch}}, revision: {{.revision}})
  build user:       {{.buildUser}}
  build date:       {{.buildDate}}
  go version:       {{.goVersion}}
`

// Print returns version information.
func Print(program string) (string, error) {
	data := map[string]string{
		"program":   program,
		"version":   Version,
		"revision":  Revision,
		"branch":    Branch,
		"buildUser": BuildUser,
		"buildDate": BuildDate,
		"goVersion": GoVersion,
	}

	var buf bytes.Buffer

	tmpl := template.Must(template.New("version").Parse(versionInfoTmpl))
	tmpl.ExecuteTemplate(&buf, "version", data)

	return strings.TrimSpace(buf.String()), nil
}

// Info returns version, branch and revision information.
func Info() []slog.Attr {
	return []slog.Attr{
		slog.String("version", Version),
		slog.String("branch", Branch),
		slog.String("revision", Revision),
	}
}

// BuildContext returns goVersion, buildUser and buildDate information.
func BuildContext() []slog.Attr {
	return []slog.Attr{
		slog.String("go", GoVersion),
		slog.String("user", BuildUser),
		slog.String("date", BuildDate),
	}
}
