package paymentprovider

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ResolvedField is a request field ready for template emission.
type ResolvedField struct {
	GoName    string
	GoExpr    string
	Type      string
	JSON      string
	OmitEmpty bool
	Redacted  bool
	IsCard    bool
	IsAPM     bool
}

// ResolveRequestFields maps CUE field sources to Go expressions.
func ResolveRequestFields(fields []RequestField, paymentSource string, currencyISONum int) ([]ResolvedField, error) {
	out := make([]ResolvedField, 0, len(fields))
	for _, f := range fields {
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
			Redacted:  f.Redacted,
			IsCard:    isCard,
			IsAPM:     isAPM,
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
		return `userID(ps)`, false, false, nil

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
		expr := fmt.Sprintf(`helpers.MapGet(%s, %s, %s)`, mapExpr, ownerKey, quote(defOr(f.Default, "")))
		return expr, from == "card", from == "apm", nil
	}
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
		"receiver_iban": "payment_providers.ReceiverIbanKey",
	}
	if c, ok := consts[key]; ok {
		return c
	}
	return quote(key)
}

var ownerInfoParams = map[string]string{
	"email":         "Email",
	"phone":         "Phone",
	"first_name":    "FirstName",
	"last_name":     "LastName",
	"fullname":      "FullName",
	"cardholder":    "CardHolder",
	"address":       "Address",
	"city":          "City",
	"state":         "State",
	"country":       "Country",
	"zip":           "Zip",
	"ip":            "IP",
	"language":      "Language",
	"birth_date":    "BirthDate",
	"receiver_iban": "ReceiverIban", // not in owner_info.go as var - use MapGet fallback
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

// BuildRequestLiteralConsts collects unique request literal constants for datatypes.go.
func BuildRequestLiteralConsts(spec *ProviderSpec) ([]RequestLiteralConst, error) {
	if spec == nil {
		return nil, nil
	}
	var fields []RequestField
	collect := func(def *RequestDef) {
		if def != nil {
			fields = append(fields, def.Fields...)
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
		"card_exp_year_short", "cardholder", "first_name", "last_name",
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
