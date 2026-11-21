package runner

import (
	"encoding/xml"
	"io"
	"strconv"
	"time"
)

type junitSuite struct {
	XMLName    xml.Name        `xml:"testsuite"`
	Name       string          `xml:"name,attr"`
	Tests      int             `xml:"tests,attr"`
	Failures   int             `xml:"failures,attr"`
	Errors     int             `xml:"errors,attr"`
	Skipped    int             `xml:"skipped,attr"`
	Time       string          `xml:"time,attr"`
	Properties []junitProperty `xml:"properties>property"`
	Cases      []junitCase     `xml:"testcase"`
}

type junitProperty struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

type junitCase struct {
	Name      string      `xml:"name,attr"`
	ClassName string      `xml:"classname,attr"`
	Time      string      `xml:"time,attr"`
	Failure   *junitIssue `xml:"failure,omitempty"`
	Error     *junitIssue `xml:"error,omitempty"`
	Skipped   *junitIssue `xml:"skipped,omitempty"`
}

type junitIssue struct {
	Message Code `xml:"message,attr"`
}

// WriteJUnit encodes only projected report metadata. It uses positional case
// names and fixed diagnostic codes instead of collection or assertion text.
func WriteJUnit(out io.Writer, report Report) error {
	suite := junitSuite{
		Name: "reva-run", Tests: len(report.Requests), Time: junitSeconds(report.DurationNS),
		Properties: []junitProperty{{Name: "outcome", Value: string(safeOutcome(report.Outcome))}},
		Cases:      make([]junitCase, 0, len(report.Requests)),
	}
	if report.Iterations > 0 {
		suite.Properties = append(suite.Properties, junitProperty{Name: "iterations", Value: strconv.Itoa(report.Iterations)})
	}
	for index, request := range report.Requests {
		entry := junitCase{Name: "request-" + strconv.Itoa(index+1), ClassName: "reva-run", Time: junitSeconds(request.DurationNS)}
		if request.Iteration > 0 {
			entry.ClassName += ".iteration-" + strconv.Itoa(request.Iteration)
		}
		switch safeOutcome(request.Outcome) {
		case Failed:
			suite.Failures++
			entry.Failure = &junitIssue{Message: safeCode(request.Code)}
		case Cancelled:
			suite.Errors++
			entry.Error = &junitIssue{Message: safeCode(request.Code)}
		case Skipped:
			suite.Skipped++
			entry.Skipped = &junitIssue{Message: safeCode(request.Code)}
		}
		suite.Cases = append(suite.Cases, entry)
	}
	if _, err := io.WriteString(out, xml.Header); err != nil {
		return err
	}
	if err := xml.NewEncoder(out).Encode(suite); err != nil {
		return err
	}
	_, err := io.WriteString(out, "\n")
	return err
}

func junitSeconds(nanoseconds int64) string {
	return strconv.FormatFloat(float64(max(0, nanoseconds))/float64(time.Second), 'f', 9, 64)
}
