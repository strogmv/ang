package report

import (
	"fmt"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/code"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/props"
	"github.com/strogmv/ang/internal/port"
)

// Generator generates PDF reports.
type Generator struct{}

// NewGenerator creates a new report generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateTenderReport creates a PDF report from tender report data.
func (g *Generator) GenerateTenderReport(data port.GetTenderReportResponse) ([]byte, error) {
	m := maroto.New()

	// Header
	m.AddRows(
		row.New(20).Add(
			col.New(12).Add(
				text.New("TENDER FINAL REPORT", props.Text{
					Align: align.Center,
					Size:  20,
					Style: fontstyle.Bold,
				}),
			),
		),
		row.New(10).Add(
			col.New(12).Add(
				text.New(fmt.Sprintf("Tender: %s (#%d)", data.TenderTitle, data.TenderNumber), props.Text{
					Align: align.Center,
					Size:  12,
				}),
			),
		),
	)

	// Winner Section
	m.AddRows(
		row.New(15).Add(
			col.New(12).Add(
				text.New("WINNER", props.Text{Style: fontstyle.Bold, Top: 5}),
			),
		),
		row.New(10).Add(
			col.New(6).Add(text.New("Company Name:")),
			col.New(6).Add(text.New(data.WinnerCompanyName, props.Text{Style: fontstyle.Bold})),
		),
	)

	// Participants Table
	m.AddRows(
		row.New(15).Add(
			col.New(12).Add(
				text.New("PARTICIPANTS", props.Text{Style: fontstyle.Bold, Top: 5}),
			),
		),
	)

	for _, p := range data.Participants {
		m.AddRows(
			row.New(10).Add(
				col.New(4).Add(text.New(p.ParticipantCode)),
				col.New(6).Add(text.New(p.CompanyName)),
				col.New(2).Add(text.New(p.Status)),
			),
		)
	}

	// QR Code for Verification
	if data.ReportID != "" {
		m.AddRows(
			row.New(40).Add(
				col.New(4).Add(
					code.NewQr(fmt.Sprintf("https://tenders.example.com/reports/%s", data.ReportID), props.Rect{
						Percent: 100,
					}),
				),
				col.New(8).Add(
					text.New("Scan to verify report authenticity online.", props.Text{
						Top: 15,
					}),
				),
			),
		)
	}

	doc, err := m.Generate()
	if err != nil {
		return nil, err
	}

	return doc.GetBytes(), nil
}
