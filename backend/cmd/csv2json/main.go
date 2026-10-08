// Command csv2json converts a Company Consensus survey sheet (CSV) into the
// JSON body accepted by the admin "Bulk import from JSON" panel and by
// POST /api/admin/games/{code}/questions/import.
//
// Usage:
//
//	go run ./cmd/csv2json [-o questions.json] survey.csv
//
// The CSV needs a header row with a "Question" column and "Option N" /
// "Points N" columns for N = 1..5. Column order does not matter and extra
// columns (e.g. "Question ID") are ignored. Fully blank rows are skipped.
//
// Every row is checked against the same rules the server applies, and all
// problems are reported at once with their CSV line number, so a bad sheet
// can be fixed in one pass rather than one paste-and-retry at a time.
package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// These mirror pollOptionCount and maxImportQuestions in internal/api/poll.go.
const (
	answersPerQuestion = 5
	maxQuestions       = 50
)

type answer struct {
	Text   string `json:"text"`
	Points int    `json:"points"`
}

type question struct {
	Text    string   `json:"text"`
	Answers []answer `json:"answers"`
}

type importBody struct {
	Questions []question `json:"questions"`
}

// columns holds the index of each column the converter reads.
type columns struct {
	question int
	option   [answersPerQuestion]int
	points   [answersPerQuestion]int
}

func main() {
	out := flag.String("o", "", "write JSON to this file instead of stdout")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "usage: csv2json [-o out.json] survey.csv\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	if err := run(flag.Arg(0), *out); err != nil {
		fmt.Fprintln(os.Stderr, "csv2json:", err)
		os.Exit(1)
	}
}

func run(inPath, outPath string) error {
	in, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer in.Close()

	body, err := convert(in)
	if err != nil {
		return err
	}

	var w io.Writer = os.Stdout
	if outPath != "" {
		f, err := os.Create(outPath)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	if err := writeJSON(w, body); err != nil {
		return err
	}
	if outPath != "" {
		fmt.Fprintf(os.Stderr, "wrote %d questions to %s\n", len(body.Questions), outPath)
	}
	return nil
}

func writeJSON(w io.Writer, body importBody) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	// Answers like "Q&A" or "<3" should read the same in the file as in the sheet.
	enc.SetEscapeHTML(false)
	return enc.Encode(body)
}

// convert reads the whole CSV and returns the import body, or one error that
// lists every problem found.
func convert(r io.Reader) (importBody, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // spreadsheet exports often have ragged rows

	header, err := cr.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return importBody{}, errors.New("file is empty")
		}
		return importBody{}, err
	}
	cols, err := findColumns(header)
	if err != nil {
		return importBody{}, err
	}

	var body importBody
	var problems []string
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return importBody{}, err
		}
		if isBlank(rec) {
			continue
		}
		line, _ := cr.FieldPos(0)
		q, errs := parseRow(rec, cols)
		for _, e := range errs {
			problems = append(problems, fmt.Sprintf("line %d: %s", line, e))
		}
		if len(errs) == 0 {
			body.Questions = append(body.Questions, q)
		}
	}

	if len(problems) == 0 && len(body.Questions) == 0 {
		return importBody{}, errors.New("no questions found")
	}
	if len(body.Questions) > maxQuestions {
		problems = append(problems, fmt.Sprintf("too many questions: %d (max %d)", len(body.Questions), maxQuestions))
	}
	if len(problems) > 0 {
		return importBody{}, fmt.Errorf("%d problem(s):\n  %s", len(problems), strings.Join(problems, "\n  "))
	}
	return body, nil
}

func findColumns(header []string) (columns, error) {
	idx := map[string]int{}
	for i, h := range header {
		// Excel and Google Sheets may prefix the first cell with a UTF-8 BOM.
		h = strings.TrimPrefix(h, "\ufeff")
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	var c columns
	var missing []string
	lookup := func(name string) int {
		i, ok := idx[strings.ToLower(name)]
		if !ok {
			missing = append(missing, fmt.Sprintf("%q", name))
		}
		return i
	}
	c.question = lookup("Question")
	for n := range answersPerQuestion {
		c.option[n] = lookup(fmt.Sprintf("Option %d", n+1))
		c.points[n] = lookup(fmt.Sprintf("Points %d", n+1))
	}
	if len(missing) > 0 {
		return columns{}, fmt.Errorf("header is missing column(s) %s", strings.Join(missing, ", "))
	}
	return c, nil
}

func parseRow(rec []string, c columns) (question, []string) {
	cell := func(i int) string {
		if i >= len(rec) {
			return ""
		}
		return strings.TrimSpace(rec[i])
	}

	var errs []string
	q := question{Text: cell(c.question)}
	if q.Text == "" {
		errs = append(errs, "question text is empty")
	}

	seen := map[string]bool{}
	for n := range answersPerQuestion {
		text := cell(c.option[n])
		rawPoints := cell(c.points[n])
		switch {
		case text == "":
			errs = append(errs, fmt.Sprintf("Option %d is empty", n+1))
		case seen[strings.ToLower(text)]:
			errs = append(errs, fmt.Sprintf("Option %d %q is a duplicate", n+1, text))
		}
		seen[strings.ToLower(text)] = true

		points, err := strconv.Atoi(rawPoints)
		switch {
		case err != nil:
			errs = append(errs, fmt.Sprintf("Points %d %q is not a whole number", n+1, rawPoints))
		case points < 0:
			errs = append(errs, fmt.Sprintf("Points %d must not be negative", n+1))
		}
		q.Answers = append(q.Answers, answer{Text: text, Points: points})
	}
	return q, errs
}

func isBlank(rec []string) bool {
	for _, f := range rec {
		if strings.TrimSpace(f) != "" {
			return false
		}
	}
	return true
}
