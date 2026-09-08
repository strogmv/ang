package schema

// ============================================================================
// UI HINTS - Abstract UI metadata for frontend generation
// ============================================================================
// These hints are UI-framework agnostic. Templates map them to MUI/shadcn/etc.
// ============================================================================

#UIType: "text" | "textarea" | "number" | "date" | "datetime" | "time" |
         "select" | "autocomplete" | "checkbox" | "switch" | "radio" |
         "file" | "image" | "currency" | "phone" | "email" | "url" | "password" | "custom"

#UIImportance: "high" | "normal" | "low"
#UIInputKind:  "sensitive" | "email" | "phone" | "money" | "search" | "none"
#UIIntent:     "danger" | "warning" | "success" | "info" | "neutral"
#UIDensity:    "compact" | "normal" | "spacious"
#UILabelMode:  "static" | "floating" | "hidden"
#UISurface:    "paper" | "flat" | "raised"

#UIHints: {
	// Display
	type?:        #UIType
	label?:       string
	placeholder?: string
	helperText?:  string
	importance?:  #UIImportance
	inputKind?:   #UIInputKind
	intent?:      #UIIntent
	density?:     #UIDensity
	labelMode?:   #UILabelMode
	surface?:     #UISurface

	// Layout
	order?:     int
	hidden?:    bool
	disabled?:  bool
	fullWidth?: bool | *true
	columns?:   int
	section?:   string

	// Type-specific
	rows?:      int      // textarea
	min?:       number   // number/currency
	max?:       number   // number/currency
	step?:      number   // number
	currency?: string   // currency (default: "BYN")
	component?: string   // type=custom component name

	// Select/Autocomplete
	source?:   string      // entity name for async options (e.g., "categories")
	options?:  [...string] // static options
	multiple?: bool

	// File/Image
	accept?:   string   // e.g., "image/*", ".pdf"
	maxSize?:  int      // bytes
}

// ============================================================================
// FORM GENERATION CONFIG
// ============================================================================

#FormConfig: {
	name:       string
	operation:  string  // CUE operation name
	submitLabel?: string
	cancelLabel?: string
	layout?:    "vertical" | "horizontal" | "grid"
	columns?:   int     // for grid layout
}

// ============================================================================
// TABLE INTENT (consumed by the frontend table emitter)
// ============================================================================
// Generated list tables read their UI intent from CUE, not from hand edits:
//
//   ListThings: schema.#Operation & {
//       @table(hidden="id,internalRef", storageKey="things")  // operation-level
//       output: { data: [...{
//           id:    string @ui(hidden)            // hidden by default, pickable
//           name:  string @ui(label="Название")  // column header
//           ...
//       }] }
//   }
//
// @table(...) keys (all optional):
//   picker=false      — no column-picker toolbar (default: picker on)
//   hidden="a,b"      — columns hidden by default (JSON field names)
//   storageKey="key"  — localStorage namespace for the user's column choice
//                       (default: generated table name)
//
// #TableConfig below is the richer, entity-level form (search/filters); it is
// reserved and not yet consumed by the emitter — prefer @table/@ui above.

#TableConfig: {
	name:       string
	operation:  string  // List operation name
	columns:    [...#ColumnConfig]
	actions?:   [...#TableAction]
	pagination?: bool
	search?:    bool
	filters?:   [...string]  // field names
}

#ColumnConfig: {
	field:      string
	header?:    string
	width?:     int
	sortable?:  bool
	filterable?: bool
	render?:    "text" | "date" | "currency" | "status" | "avatar" | "link"
}

#TableAction: {
	name:   string
	icon?:  string
	label?: string
	action: "view" | "edit" | "delete" | "custom"
	href?:  string  // for view/edit links
}

// ============================================================================
// RESOURCE CRUD CONFIG
// ============================================================================

#CRUDConfig: {
	enabled: bool | *false
	custom?: bool | *false // If true, ANG won't overwrite generated files
	views?: {
		list?:    bool | *true
		details?: bool | *true
		create?:  bool | *true
		edit?:    bool | *true
	}
	permissions?: {
		list?:   string
		get?:    string
		create?: string
		update?: string
		delete?: string
	}
}

#UI: {
	crud?: #CRUDConfig
	list?: #TableConfig
	form?: #FormConfig
}
