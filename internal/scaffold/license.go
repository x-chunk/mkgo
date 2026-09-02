package scaffold

import (
	"bytes"
	"embed"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

//go:embed licenses/*.txt
var licenseFS embed.FS

// licenseNames maps the SPDX identifier accepted on the command line to the
// human readable name shown in the summary and the README badge.
var licenseNames = map[string]string{
	"mit":          "MIT",
	"apache-2.0":   "Apache-2.0",
	"bsd-3-clause": "BSD-3-Clause",
	"isc":          "ISC",
	"unlicense":    "Unlicense",
}

// licenseAliases lets users type the short form they remember.
var licenseAliases = map[string]string{
	"apache":    "apache-2.0",
	"apache2":   "apache-2.0",
	"apache-2":  "apache-2.0",
	"bsd":       "bsd-3-clause",
	"bsd3":      "bsd-3-clause",
	"bsd-3":     "bsd-3-clause",
	"unlicence": "unlicense",
	"none":      "",
	"":          "",
}

// NormalizeLicense resolves an alias to its canonical SPDX identifier. An
// empty result means "do not write a LICENSE file".
func NormalizeLicense(id string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(id))
	if canonical, ok := licenseAliases[key]; ok {
		key = canonical
	}
	if key == "" {
		return "", nil
	}
	if _, ok := licenseNames[key]; !ok {
		return "", fmt.Errorf("unknown license %q (available: %s, none)", id, strings.Join(AvailableLicenses(), ", "))
	}
	return key, nil
}

// AvailableLicenses lists the supported SPDX identifiers in a stable order.
func AvailableLicenses() []string {
	ids := make([]string, 0, len(licenseNames))
	for id := range licenseNames {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// LicenseName returns the display name for a canonical identifier.
func LicenseName(id string) string {
	if name, ok := licenseNames[id]; ok {
		return name
	}
	return id
}

// renderLicense expands the stored license text with the project's copyright
// holder and year.
func renderLicense(id string, data any) (string, error) {
	raw, err := licenseFS.ReadFile("licenses/" + id + ".txt")
	if err != nil {
		return "", fmt.Errorf("license %q is not embedded: %w", id, err)
	}
	tmpl, err := template.New("license").Parse(string(raw))
	if err != nil {
		return "", fmt.Errorf("parse license template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render license: %w", err)
	}
	return buf.String(), nil
}
