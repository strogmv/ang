package api

import "github.com/strogmv/ang/cue/schema"

HTTP: schema.#HTTP & {
  GenerateReportPDF: {
    method: "POST"
    path: "/reports/pdf"
  }
}
