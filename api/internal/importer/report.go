package importer

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Report says what an import did: how many rows of each kind were imported,
// and every row that was left behind or adapted, with the reason.
type Report struct {
	DryRun   bool
	Imported map[string]int
	// Skipped and Adapted map a reason to the rows it applies to.
	Skipped map[string][]string
	Adapted map[string][]string
	// LeftOut counts Cachet rows of kinds Vigie does not import at all.
	LeftOut map[string]int64
	// Notes are things to do by hand after the import.
	Notes []string
}

func newReport(dryRun bool) *Report {
	return &Report{DryRun: dryRun, Imported: map[string]int{}, Skipped: map[string][]string{},
		Adapted: map[string][]string{}, LeftOut: map[string]int64{}}
}

func (r *Report) imported(kind string)     { r.Imported[kind]++ }
func (r *Report) skip(reason, row string)  { r.Skipped[reason] = append(r.Skipped[reason], row) }
func (r *Report) adapt(reason, row string) { r.Adapted[reason] = append(r.Adapted[reason], row) }
func (r *Report) note(format string, args ...any) {
	r.Notes = append(r.Notes, fmt.Sprintf(format, args...))
}
func row(kind string, id uint, name string) string { return fmt.Sprintf("%s %d (%s)", kind, id, name) }

// Print writes the report in a readable form.
func (r *Report) Print(w io.Writer) {
	if r.DryRun {
		fmt.Fprintln(w, "DRY RUN: nothing was written. This is what an import would do.")
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w, "Imported:")
	kinds := make([]string, 0, len(r.Imported))
	for k := range r.Imported {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		fmt.Fprintf(w, "  %-26s %d\n", k, r.Imported[k])
	}
	section := func(title string, m map[string][]string) {
		if len(m) == 0 {
			return
		}
		fmt.Fprintf(w, "\n%s:\n", title)
		reasons := make([]string, 0, len(m))
		for reason := range m {
			reasons = append(reasons, reason)
		}
		sort.Strings(reasons)
		for _, reason := range reasons {
			rows := m[reason]
			fmt.Fprintf(w, "  %s: %d\n", reason, len(rows))
			shown := rows
			if len(shown) > 10 {
				shown = shown[:10]
			}
			fmt.Fprintf(w, "    %s", strings.Join(shown, ", "))
			if len(rows) > len(shown) {
				fmt.Fprintf(w, ", and %d more", len(rows)-len(shown))
			}
			fmt.Fprintln(w)
		}
	}
	section("Adapted", r.Adapted)
	section("Not imported", r.Skipped)
	if len(r.LeftOut) > 0 {
		fmt.Fprintln(w, "\nNot supported by Vigie, left in Cachet:")
		kinds := make([]string, 0, len(r.LeftOut))
		for k := range r.LeftOut {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		for _, k := range kinds {
			fmt.Fprintf(w, "  %s: %d\n", k, r.LeftOut[k])
		}
	}
	if len(r.Notes) > 0 {
		fmt.Fprintln(w, "\nTo do by hand:")
		for _, n := range r.Notes {
			fmt.Fprintf(w, "  - %s\n", n)
		}
	}
}
