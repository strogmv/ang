package paymentprovider

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ResolvedField is a request field ready for template emission. A field that
// groups others carries them in Nested and has no expression of its own.
type ResolvedField struct {
	GoName    string
	GoExpr    string
	Type      string
	JSON      string
	OmitEmpty bool
	Required  bool
	Redacted  bool
	IsCard    bool
	IsAPM     bool
	// Source is the contract's source verbatim. It is passed through so a
	// template can branch on what a value is without the generator having to
	// know what any particular source means.
	Source string
	Nested []ResolvedField
	// PerMethod marks an object filled from the selected payment method. Nested
	// then holds every field any method can put there, and Methods says which of
	// them each method actually sets.
	PerMethod bool
	Methods   []ResolvedMethod
	// Locals is set on a per-method object: it builds itself, so it declares the
	// variables its own expressions read from.
	Locals []RequestLocal
	// CtorName is the constructor of a per-method object.
	CtorName string
}

// ResolvedMethod is one payment method's contribution to a per-method object.
type ResolvedMethod struct {
	Sid           string
	ProviderValue string
	// GoConst is the identifier in utils/types (CardsMethod). Empty when the
	// method is only listed for provider_value lookup and has no destination.
	GoConst string
	// Locals are declared inside this method's branch, not at the slot
	// constructor top: a card method and an APM method must not both panic
	// the other source.
	Locals []RequestLocal
	Fields []ResolvedField
}

// IsObject reports whether the field groups other fields rather than carrying a
// value; templates emit a nested struct for it.
func (f ResolvedField) IsObject() bool { return len(f.Nested) > 0 }

// ResolveRequestFields maps CUE field sources to Go expressions.
func ResolveRequestFields(fields []RequestField, paymentSource string, currencyISONum int) ([]ResolvedField, error) {
	return ResolveRequestFieldsIn("", fields, paymentSource, currencyISONum)
}

// ResolveRequestFieldsIn is ResolveRequestFields scoped to an owning request.
// The scope only names the nested struct types: a payin and a payout may both
// carry a "customer" object with different fields, so the generated types must
// not collide on the field name alone.
func ResolveRequestFieldsIn(scope string, fields []RequestField, paymentSource string, currencyISONum int) ([]ResolvedField, error) {
	out := make([]ResolvedField, 0, len(fields))
	for _, f := range fields {
		if f.PerMethod {
			// Contents are filled from the methods once those are known; the
			// field list here is empty by definition.
			out = append(out, ResolvedField{
				GoName:    f.Name,
				Type:      goTypeForObject(scope, f.Name),
				JSON:      f.JSON,
				OmitEmpty: f.OmitEmpty,
				PerMethod: true,
			})
			continue
		}
		if len(f.Fields) > 0 {
			typeName := goTypeForObject(scope, f.Name)
			nested, err := ResolveRequestFieldsIn(typeName, f.Fields, paymentSource, currencyISONum)
			if err != nil {
				return nil, fmt.Errorf("object %s: %w", f.Name, err)
			}
			out = append(out, ResolvedField{
				GoName:    f.Name,
				Type:      typeName,
				JSON:      f.JSON,
				OmitEmpty: f.OmitEmpty,
				Nested:    nested,
			})
			continue
		}
		expr, isCard, isAPM, err := resolveSource(f, paymentSource, currencyISONum)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", f.Name, err)
		}
		typ := strings.TrimSpace(f.Type)
		if typ == "" {
			typ = inferGoType(f.Source, currencyISONum)
		}
		out = append(out, ResolvedField{
			GoName:    f.Name,
			GoExpr:    expr,
			Type:      typ,
			JSON:      f.JSON,
			OmitEmpty: f.OmitEmpty,
			Required:  f.Required,
			Redacted:  f.Redacted,
			IsCard:    isCard,
			IsAPM:     isAPM,
			Source:    strings.TrimSpace(f.Source),
		})
	}
	return out, nil
}

func resolveSource(f RequestField, paymentSource string, currencyISONum int) (expr string, isCard, isAPM bool, err error) {
	src := strings.TrimSpace(f.Source)
	def := strings.TrimSpace(f.Default)

	switch src {
	case "tx_id":
		return `tx.Id`, false, false, nil
	case "foreign_id":
		return `tx.ForeignId`, false, false, nil
	case "tx_amount":
		return `tx.Amount`, false, false, nil
	case "tx_amount_float":
		return `helpers.FloatAmount(tx.Amount, tx.Currency)`, false, false, nil
	case "tx_amount_fmt":
		return `helpers.CoinsToFormattedAmount(tx.Amount, tx.Currency)`, false, false, nil
	case "tx_amount_sprintf":
		return `fmt.Sprintf("%.2f", helpers.FloatAmount(tx.Amount, tx.Currency))`, false, false, nil
	case "tx_currency":
		return `tx.Currency`, false, false, nil
	case "tx_callback_url":
		return `tx.CallbackURL`, false, false, nil
	case "tx_ip":
		return `tx.Order.IP`, false, false, nil
	case "tx_description":
		return `tx.Order.Description`, false, false, nil
	case "tx_result_url":
		return `tx.Order.ResultUrl`, false, false, nil
	case "tx_merchant_order":
		return `tx.Order.MerchantOrderId`, false, false, nil
	case "tx_card_country":
		return `tx.Order.CardCountry`, false, false, nil
	case "tx_payment_method":
		return `tx.Order.PaymentMethodBrand`, false, false, nil

	case "card_pan":
		return `card.PAN`, true, false, nil
	case "card_cvv":
		return `card.CVV`, true, false, nil
	case "card_exp_month":
		return `card.ExpDateMonth`, true, false, nil
	case "card_exp_year":
		return `card.ExpDateYear`, true, false, nil
	case "card_exp_month_fmt":
		return `fmt.Sprintf("%02d", card.ExpDateMonth)`, true, false, nil
	case "card_exp_year_fmt":
		return `fmt.Sprintf("%d", card.ExpDateYear)`, true, false, nil
	case "card_exp_year_short":
		return `fmt.Sprintf("%02d", card.ExpDateYear-2000)`, true, false, nil
	case "cardholder":
		return mapGet("card.OwnerInfo", "cardholder", def), true, false, nil
	case "first_name":
		return `firstName`, true, false, nil
	case "last_name":
		return `lastName`, true, false, nil
	case "card_email":
		return mapGet("card.OwnerInfo", "email", def), true, false, nil
	case "card_phone":
		return mapGet("card.OwnerInfo", "phone", def), true, false, nil
	case "card_customer_id":
		return `card.OwnerInfo["ExternalCustomerID"]`, true, false, nil

	case "apm_id":
		return `ps.APM.ID`, false, true, nil
	case "apm_id_type":
		return `ps.APM.IDType`, false, true, nil
	case "apm_name":
		return mapGet("info", "name", defOr(def, "John Doe")), false, true, nil
	case "apm_email":
		return mapGet("info", "email", def), false, true, nil
	case "apm_phone":
		return mapGet("info", "phone", def), false, true, nil
	case "apm_first_name":
		return mapGet("info", "first_name", def), false, true, nil
	case "apm_last_name":
		return mapGet("info", "last_name", def), false, true, nil

	case "owner_info":
		return resolveOwnerInfo(f, paymentSource)

	case "mc_mid":
		return `mc.Mid`, false, false, nil
	case "mc_endpoint":
		return `mc.ApiEndpoint`, false, false, nil
	case "secret":
		key := strings.TrimSpace(f.SecretKey)
		if key == "" {
			return "", false, false, fmt.Errorf("secret_key required for source \"secret\"")
		}
		return fmt.Sprintf("secrets.%s", exportGoIdent(key)), false, false, nil

	case "browser_language":
		return browserData("BrowserLanguage", defOr(def, "en")), false, false, nil
	case "browser_color_depth":
		return browserData("BrowserScreenColorDepth", defOr(def, "24")), false, false, nil
	case "browser_screen_height":
		return browserData("BrowserScreenHeight", defOr(def, "900")), false, false, nil
	case "browser_screen_width":
		return browserData("BrowserScreenWidth", defOr(def, "1440")), false, false, nil
	case "browser_window_width":
		return browserData("BrowserWindowWidth", defOr(def, "800")), false, false, nil
	case "browser_window_height":
		return browserData("BrowserWindowHeight", defOr(def, "600")), false, false, nil
	case "browser_java_enabled":
		return browserData("BrowserJavaEnabled", defOr(def, "false")), false, false, nil
	case "browser_user_agent":
		return browserData("BrowserUserAgent", defOr(def, "Mozilla/5.0")), false, false, nil
	case "browser_tz":
		return `helpers.TimezoneOffset(ps.GetBrowserData(model.BrowserTimezone, ` + quote(defOr(def, "UTC")) + `))`, false, false, nil
	case "browser_tz_name":
		return browserData("BrowserTimezone", defOr(def, "UTC")), false, false, nil
	case "browser_accept":
		return browserData("BrowserAcceptHeader", defOr(def, "text/html,application/xhtml+xml")), false, false, nil

	case "provider_method_value":
		return `providerMethodValue(tx)`, false, false, nil
	case "salt":
		return `generateSalt()`, false, false, nil
	case "currency_iso_num":
		return strconv.Itoa(currencyISONum), false, false, nil
	case "const":
		v := strings.TrimSpace(f.ConstVal)
		if v == "" {
			return "", false, false, fmt.Errorf("const_val required for source \"const\"")
		}
		return RequestLiteralConstName(f), false, false, nil

	case "uuid":
		return `helpers.UUID()`, false, false, nil
	case "client_payout_id":
		return `clientPayoutID`, false, false, nil
	case "card_holder_name":
		return `strings.TrimSpace(firstName + " " + lastName)`, true, false, nil
	case "description_payment":
		return `fmt.Sprintf("Payment_%s", tx.Id)`, false, false, nil
	case "description_payout":
		return `fmt.Sprintf("Payout_%s", tx.Id)`, false, false, nil
	case "notification_token":
		key := strings.TrimSpace(f.SecretKey)
		if key == "" {
			key = "secret"
		}
		return fmt.Sprintf("computeNotificationToken(tx.Id, secrets.%s)", exportGoIdent(key)), false, false, nil
	case "utc_timestamp":
		return `pp.utcTimestamp()`, false, false, nil
	case "card_exp_last_day":
		return `func() string { if card.ExpDateMonth == 0 || card.ExpDateYear == 0 { return "" }; return card.ExpTime().AddDate(0, 0, -1).Format("2006-01-02") }()`, true, false, nil
	case "external_customer_id":
		switch paymentSource {
		case "card":
			return `card.OwnerInfo[providers.ExternalCustomerIDKey]`, true, false, nil
		case "apm":
			return `info[providers.ExternalCustomerIDKey]`, false, true, nil
		default:
			return `ownerInfo(ps)[providers.ExternalCustomerIDKey]`, false, false, nil
		}

	default:
		return "", false, false, fmt.Errorf("unknown source %q", src)
	}
}

func resolveOwnerInfo(f RequestField, paymentSource string) (string, bool, bool, error) {
	key := strings.TrimSpace(f.OwnerKey)
	if key == "" {
		return "", false, false, fmt.Errorf("owner_key required for source \"owner_info\"")
	}
	param, ok := ownerInfoParams[key]
	from := strings.TrimSpace(f.OwnerFrom)
	if from == "" {
		switch paymentSource {
		case "card":
			from = "card"
		default:
			from = "apm"
		}
	}
	mapExpr := "info"
	if from == "card" {
		mapExpr = "card.OwnerInfo"
	}
	if !ok {
		ownerKey := ownerInfoKeyConst(key)
		if def := strings.TrimSpace(f.Default); def != "" {
			expr := fmt.Sprintf(`helpers.MapGet(%s, %s, %s)`, mapExpr, ownerKey, quote(def))
			return expr, from == "card", from == "apm", nil
		}
		// No OwnerInfoParameter and no sentinel default: map index is the same
		// as MapGet(..., "") and is what reviewers expect.
		return fmt.Sprintf(`%s[%s]`, mapExpr, ownerKey), from == "card", from == "apm", nil
	}
	// OwnerInfoParameter keys always go through GetParameter, including omitempty:
	// omitempty only affects the JSON tag. Filling the platform default when
	// randomization is off is the customer-data contract; wrapping GetParameter
	// in MapGet(..., "") is never generated.
	switch from {
	case "card":
		return fmt.Sprintf("providers.GetParameter(card.OwnerInfo, providers.%s)", param), true, false, nil
	case "apm":
		return fmt.Sprintf("providers.GetParameter(info, providers.%s)", param), false, true, nil
	default:
		return "", false, false, fmt.Errorf("owner_from must be card|apm, got %q", from)
	}
}

func ownerInfoKeyConst(key string) string {
	consts := map[string]string{
		"receiver_iban": "providers.ReceiverIbanKey",
	}
	if c, ok := consts[key]; ok {
		return c
	}
	return quote(key)
}

var ownerInfoParams = map[string]string{
	"email":      "Email",
	"phone":      "Phone",
	"first_name": "FirstName",
	"last_name":  "LastName",
	"fullname":   "FullName",
	"cardholder": "CardHolder",
	"address":    "Address",
	"city":       "City",
	"state":      "State",
	"country":    "Country",
	"zip":        "Zip",
	"ip":         "IP",
	"language":   "Language",
	"birth_date": "BirthDate",
}

func inferGoType(source string, currencyISONum int) string {
	switch source {
	case "tx_amount", "card_exp_month", "card_exp_year", "currency_iso_num":
		if source == "currency_iso_num" {
			return "int"
		}
		if strings.HasPrefix(source, "card_exp") {
			return "int"
		}
		return "int64"
	case "tx_amount_float":
		return "float64"
	case "browser_color_depth", "browser_screen_height", "browser_screen_width",
		"browser_window_width", "browser_window_height":
		return "int"
	default:
		return "string"
	}
}

func mapGet(mapExpr, key, def string) string {
	if def == "" {
		return fmt.Sprintf(`helpers.MapGet(%s, %q, "")`, mapExpr, key)
	}
	return fmt.Sprintf(`helpers.MapGet(%s, %q, %q)`, mapExpr, key, def)
}

func browserData(field, def string) string {
	return fmt.Sprintf(`ps.GetBrowserData(model.%s, %s)`, field, quote(def))
}

func quote(s string) string {
	return strconv.Quote(s)
}

// RequestLiteralConstName is the Go identifier for a request field with source "const".
func RequestLiteralConstName(f RequestField) string {
	if n := strings.TrimSpace(f.ConstName); n != "" {
		return n
	}
	return exportGoIdent(f.Name)
}

// constructorName is the Go name of a generated constructor for a type.
func constructorName(typeName string) string {
	if typeName == "" {
		return ""
	}
	return "new" + strings.ToUpper(typeName[:1]) + typeName[1:]
}

// RequestLocal is a variable a request body's expressions read from.
type RequestLocal struct {
	Name string
	Decl string
}

// requestLocalDecls are the locals resolved expressions may refer to, in the
// order they have to be declared: the cardholder split reads the card.
var requestLocalDecls = []RequestLocal{
	{Name: "card", Decl: "card := ps.Card"},
	{Name: "info", Decl: "info := ownerInfo(ps)"},
	{Name: "firstName", Decl: "firstName, lastName := helpers.SplitCardHolder(card.OwnerInfo)"},
}

// RequestLocals reports which locals a set of fields needs declared before the
// body can be built. It reads the expressions rather than the sources, so a new
// source that reuses an existing local is covered without being listed twice.
func RequestLocals(fields []ResolvedField) []RequestLocal {
	used := map[string]bool{}
	var scan func([]ResolvedField)
	scan = func(fs []ResolvedField) {
		for _, f := range fs {
			// A per-method object builds itself, and declares its own locals there.
			if f.PerMethod {
				continue
			}
			if f.IsObject() {
				scan(f.Nested)
				continue
			}
			for _, l := range requestLocalDecls {
				if referencesIdent(f.GoExpr, l.Name) {
					used[l.Name] = true
				}
			}
			if referencesIdent(f.GoExpr, "lastName") {
				used["firstName"] = true
			}
		}
	}
	scan(fields)
	if used["firstName"] {
		used["card"] = true
	}
	var out []RequestLocal
	for _, l := range requestLocalDecls {
		if used[l.Name] {
			out = append(out, l)
		}
	}
	return out
}

// referencesIdent reports whether expr uses ident as a whole identifier, so
// "card" does not match "cardHolder".
func referencesIdent(expr, ident string) bool {
	for i := 0; i+len(ident) <= len(expr); i++ {
		if expr[i:i+len(ident)] != ident {
			continue
		}
		if i > 0 && isIdentByte(expr[i-1]) {
			continue
		}
		if end := i + len(ident); end < len(expr) && isIdentByte(expr[end]) {
			continue
		}
		return true
	}
	return false
}

func isIdentByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// FillPerMethodObjects resolves the method-specific slots of a request. Every
// method's destination is resolved against the object's own type name, then
// merged: the struct carries the union of what any method can send, and each
// method keeps the subset it actually sets. Fields common to several methods
// must agree — the same JSON name cannot mean two different things in one body.
func FillPerMethodObjects(fields []ResolvedField, methods []Method, paymentSource string, currencyISONum int) error {
	for i := range fields {
		f := &fields[i]
		if !f.PerMethod {
			if err := FillPerMethodObjects(f.Nested, methods, paymentSource, currencyISONum); err != nil {
				return err
			}
			continue
		}
		if len(methods) == 0 {
			return fmt.Errorf("object %s is per_method but the provider declares no methods", f.GoName)
		}
		union := make([]ResolvedField, 0)
		seen := map[string]ResolvedField{}
		for _, m := range methods {
			resolved, err := ResolveRequestFieldsIn(f.Type, m.Destination, paymentSource, currencyISONum)
			if err != nil {
				return fmt.Errorf("method %s: %w", m.Sid, err)
			}
			// These fields are resolved after the request's own were refined, so
			// they need the same treatment or owner_from would be ignored here.
			refineOwnerSource(resolved, m.Destination, paymentSource)
			for _, rf := range resolved {
				prev, ok := seen[rf.GoName]
				if !ok {
					seen[rf.GoName] = rf
					union = append(union, rf)
					continue
				}
				if prev.JSON != rf.JSON || prev.Type != rf.Type {
					return fmt.Errorf("field %s of object %s differs between methods: %s %s vs %s %s",
						rf.GoName, f.GoName, prev.Type, prev.JSON, rf.Type, rf.JSON)
				}
			}
			goConst, err := paymentMethodGoConst(m.Sid)
			if err != nil {
				return err
			}
			f.Methods = append(f.Methods, ResolvedMethod{
				Sid:           m.Sid,
				ProviderValue: m.ProviderValue,
				GoConst:       goConst,
				Locals:        RequestLocals(resolved),
				Fields:        resolved,
			})
		}
		// A method sends only its own fields, so every one of them has to be
		// omissible for the others — unless the contract marked it required,
		// in which case an empty value is still meaningful on the wire.
		for j := range union {
			if !union[j].Required {
				union[j].OmitEmpty = true
			}
		}
		f.Nested = union
		f.CtorName = constructorName(f.Type)
	}
	return nil
}

// flattenRequestFields returns every leaf field of a request, whatever depth it
// sits at. Anything that inspects fields by source has to see grouped bodies
// too, or a contract that nests its fields loses them silently.
func flattenRequestFields(fields []RequestField) []RequestField {
	out := make([]RequestField, 0, len(fields))
	for _, f := range fields {
		if len(f.Fields) > 0 {
			out = append(out, flattenRequestFields(f.Fields)...)
			continue
		}
		out = append(out, f)
	}
	return out
}

// BuildRequestLiteralConsts collects unique request literal constants for datatypes.go.
func BuildRequestLiteralConsts(spec *ProviderSpec) ([]RequestLiteralConst, error) {
	if spec == nil {
		return nil, nil
	}
	var fields []RequestField
	collect := func(def *RequestDef) {
		if def != nil {
			fields = append(fields, flattenRequestFields(def.Fields)...)
		}
	}
	collect(spec.PayinRequest)
	collect(spec.PayoutRequest)
	collect(spec.PayinStatusRequest)
	collect(spec.PayoutStatusRequest)
	collect(spec.RefundRequest)
	collect(spec.P2PRequest)

	seen := map[string]string{}
	for _, f := range fields {
		if strings.TrimSpace(f.Source) != "const" {
			continue
		}
		v := strings.TrimSpace(f.ConstVal)
		if v == "" {
			continue
		}
		name := RequestLiteralConstName(f)
		if prev, ok := seen[name]; ok && prev != v {
			return nil, fmt.Errorf("request const %q has conflicting values %q and %q", name, prev, v)
		}
		seen[name] = v
	}
	if len(seen) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]RequestLiteralConst, 0, len(names))
	for _, name := range names {
		out = append(out, RequestLiteralConst{Name: name, Value: seen[name]})
	}
	return out, nil
}

func defOr(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func exportGoIdent(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = []rune(string(runes[0]))[0] // no-op safety
	// lower first char for secrets struct fields (merchantUUID style)
	if runes[0] >= 'A' && runes[0] <= 'Z' {
		runes[0] += 'a' - 'A'
	}
	return string(runes)
}

// IsCardSource reports whether a field source is card-scoped.
func IsCardSource(source string) bool {
	switch source {
	case "card_pan", "card_cvv", "card_exp_month", "card_exp_year", "card_exp_month_fmt",
		"card_exp_year_fmt", "card_exp_year_short", "cardholder", "first_name", "last_name",
		"card_email", "card_phone", "card_customer_id", "card_exp_last_day", "card_holder_name":
		return true
	default:
		return false
	}
}

// IsAPMSource reports whether a field source is APM-scoped.
func IsAPMSource(source string) bool {
	switch source {
	case "apm_id", "apm_id_type", "apm_name", "apm_email", "apm_phone", "apm_first_name", "apm_last_name":
		return true
	case "owner_info":
		return true // may be card or apm; template filters by OwnerFrom
	default:
		return false
	}
}

// goTypeForObject names the struct generated for a nested request object. The
// name is derived from the field so a contract never has to invent Go types.
func goTypeForObject(scope, fieldName string) string {
	if fieldName == "" {
		return ""
	}
	if scope == "" {
		return strings.ToLower(fieldName[:1]) + fieldName[1:] + "Object"
	}
	return scope + strings.ToUpper(fieldName[:1]) + fieldName[1:]
}

// RequestTypeScope is the prefix nested types of a request are named under. The
// definition name already reads as the body it describes ("payoutRequest"), so
// the trailing noun is dropped to keep payoutCustomer from becoming
// payoutRequestCustomer.
func RequestTypeScope(defName string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(defName), "Request")
	if trimmed == "" {
		return strings.TrimSpace(defName)
	}
	return trimmed
}

// mixedConstructorLocals reports a request whose envelope (not a per-method
// slot) needs both a card and an APM local. Those sources cannot share one
// constructor: only one of ps.Card and ps.APM is live.
func mixedConstructorLocals(locals []RequestLocal) error {
	hasCard, hasAPM := false, false
	for _, l := range locals {
		switch l.Name {
		case "card", "firstName":
			hasCard = true
		case "info":
			hasAPM = true
		}
	}
	if hasCard && hasAPM {
		return fmt.Errorf("constructor mixes card and APM sources; put method-specific fields in a per_method slot")
	}
	return nil
}

// MaskLeaf is one value-carrying field addressed from a request String()
// receiver. The selector is a Go expression (r.Customer.Email); which fields
// are actually masked remains a template decision.
type MaskLeaf struct {
	Selector string
	Source   string
	Type     string
	Redacted bool
}

func collectObjectTypes(fields []ResolvedField) []ResolvedField {
	var out []ResolvedField
	var walk func([]ResolvedField)
	walk = func(fs []ResolvedField) {
		for _, f := range fs {
			if f.PerMethod || f.IsObject() {
				out = append(out, f)
				walk(f.Nested)
			}
		}
	}
	walk(fields)
	return out
}

func collectMaskLeaves(fields []ResolvedField, prefix string) []MaskLeaf {
	var out []MaskLeaf
	for _, f := range fields {
		sel := prefix + "." + f.GoName
		if f.PerMethod || f.IsObject() {
			out = append(out, collectMaskLeaves(f.Nested, sel)...)
			continue
		}
		out = append(out, MaskLeaf{
			Selector: sel,
			Source:   f.Source,
			Type:     f.Type,
			Redacted: f.Redacted,
		})
	}
	return out
}

func fieldsUseSecret(fields []ResolvedField) bool {
	for _, f := range fields {
		if f.PerMethod {
			for _, m := range f.Methods {
				if fieldsUseSecret(m.Fields) {
					return true
				}
			}
		}
		if f.IsObject() {
			if fieldsUseSecret(f.Nested) {
				return true
			}
			continue
		}
		switch f.Source {
		case "secret", "notification_token":
			return true
		}
	}
	return false
}
