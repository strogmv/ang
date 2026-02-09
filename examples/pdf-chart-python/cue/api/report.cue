package api

import "github.com/strogmv/ang/cue/schema"

GenerateReportPDF: schema.#Operation & {
  service: "report"
  description: "Generate PDF with line chart"

  input: {
    title?: string
    points: [...{
      x: float
      y: float
    }]
  }

  output: {
    ok: bool
  }

  impl: {
    lang: "python"
    imports: [
      "from app.lib.pdf_chart import generate_pdf_report_response",
    ]
    code: """
payload = args[0] if args else kwargs.get("payload", {})
return generate_pdf_report_response(payload)
"""
  }
}
