package schema

// ============================================================================
// UI HINTS - Abstract UI metadata for frontend generation
// ============================================================================
// These hints are UI-framework agnostic. Templates map them to MUI/shadcn/etc.
// ============================================================================

#UIType: "text" | "textarea" | "number" | "date" | "datetime" | "time" |
         "select" | "autocomplete" | "checkbox" | "switch" | "radio" |
         "file" | "image" | "currency" | "phone" | "email" | "url" | "password"

#UIHints: {
	// Display
	type?:        #UIType
	label?:       string
	placeholder?: string
	helperText?:  string

	// Layout
	order?:    int
	hidden?:   bool
	disabled?: bool
	fullWidth?: bool | *true

	// Type-specific
	rows?:     int      // textarea
	min?:      number   // number/currency
	max?:      number   // number/currency
	step?:     number   // number
	currency?: string   // currency (default: "BYN")

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
