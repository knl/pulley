package version

import (
	"bytes"
	"fmt"
	"runtime"
	"strings"
	"text/template"

	"github.com/prometheus/client_golang/prometheus"
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

// NewCollector returns a collector that exports metrics about current version
// information.
func NewCollector() prometheus.Collector {
	return prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "build_info",
			Help: "A metric with a constant '1' value labeled by version, revision, branch, and goversion from which pulley was built.",
			ConstLabels: prometheus.Labels{
				"version":   Version,
				"revision":  Revision,
				"branch":    Branch,
				"goversion": GoVersion,
			},
		},
		func() float64 { return 1 },
	)
}

// versionInfoTmpl contains the template used by Info.
var versionInfoTmpl = `
pulley version {{.version}} (branch: {{.branch}}, revision: {{.revision}})
  build user:       {{.buildUser}}
  build date:       {{.buildDate}}
  go version:       {{.goVersion}}
`

// Print returns version information.
func Print() string {
	m := map[string]string{
		"version":   Version,
		"revision":  Revision,
		"branch":    Branch,
		"buildUser": BuildUser,
		"buildDate": BuildDate,
		"goVersion": GoVersion,
	}
	t := template.Must(template.New("version").Parse(versionInfoTmpl))

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "version", m); err != nil {
		panic(err)
	}

	return strings.TrimSpace(buf.String())
}

// Info returns version, branch and revision information.
func Info() string {
	return fmt.Sprintf("(version=%s, branch=%s, revision=%s)", Version, Branch, Revision)
}

// BuildContext returns goVersion, buildUser and buildDate information.
func BuildContext() string {
	return fmt.Sprintf("(go=%s, user=%s, date=%s)", GoVersion, BuildUser, BuildDate)
}
