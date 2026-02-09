package domain

import "github.com/strogmv/ang/cue/schema"

#ReportPoint: schema.#Entity & {
  name: "ReportPoint"
  owner: "report"
  description: "A point on the report chart"
  fields: {
    id: { type: "uuid" }
    x: { type: "float" }
    y: { type: "float" }
  }
}
