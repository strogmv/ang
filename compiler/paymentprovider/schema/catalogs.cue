package schema

// Catalog enums are split from provider.cue for readability.

#CatalogFieldSource:
	"tx_id" | "tx_amount" | "tx_amount_float" | "tx_amount_fmt" | "tx_amount_sprintf" |
	"tx_currency" | "tx_callback_url" | "tx_ip" | "tx_description" | "tx_result_url" |
	"tx_merchant_order" | "tx_card_country" | "tx_payment_method" |
	"card_pan" | "card_cvv" | "card_exp_month" | "card_exp_year" | "card_exp_month_fmt" |
	"card_exp_year_short" | "cardholder" | "first_name" | "last_name" |
	// Deprecated for new providers: prefer source "owner_info" + owner_key and
	// providers.GetParameter in Go so platform defaults apply when randomization is off.
	"card_email" |
	"card_phone" | "card_customer_id" | "owner_info" |
	"apm_id" | "apm_id_type" | "apm_name" | "apm_email" | "apm_phone" | "apm_first_name" | "apm_last_name" |
	"mc_mid" | "mc_endpoint" | "secret" |
	"browser_language" | "browser_color_depth" | "browser_screen_height" | "browser_screen_width" |
	"browser_window_width" | "browser_window_height" | "browser_java_enabled" | "browser_user_agent" |
	"browser_tz" | "browser_tz_name" | "browser_accept" |
	"salt" | "currency_iso_num" | "const" | "uuid" |
	"notification_token" | "utc_timestamp" | "card_exp_last_day" |
	"external_customer_id" | "description_payment" | "description_payout" |
	"client_payout_id" | "card_holder_name"

#CatalogOwnerInfoKey:
	"email" | "phone" | "first_name" | "last_name" | "fullname" | "cardholder" |
	"address" | "city" | "state" | "country" | "zip" | "ip" | "language" |
	"birth_date" | "receiver_iban"

#CatalogStatusCode:
	"SCodeOk" | "SCodeNoPaymentRoutes" | "SCodeSuspended" | "SCodeBlocked" |
	"SCodeNotSupportedAction" | "SCodeInternalError" | "SCodeCancelledByCustomer" |
	"SCodeDeclinedByAntifraud" | "SCodeDeclinedByTDS" | "SCodeTDSTimeout" |
	"SCodeDeclinedByBank" | "SCodeRequisiteNotAvailable" | "SCodeLimitReached" |
	"SCodeCardLimitReached" | "SCodeInsufficientFunds" | "SCodeIncorrectCardData" |
	"SCodeInvalidPhoneNumber" | "SCodeWaitCascading" | "SCodeCustomerLimitReached" |
	"SCodeTimeouted"

#CatalogPaymentMethodSID:
	"cards" | "visa" | "mastercard" | "amex" | "discover" | "jcb" |
	"unionpay" | "mir" | "internationalcard" | "UZCARD" | "HUMO" |
	"MBANK" | "optimabank" | "CARD" |
	"bitcoin" | "litecoin" | "bitcoincash" | "cardano" | "ethereum" |
	"ethereumclassic" | "dogecoin" | "neo" | "ripple" | "augur" |
	"tether" | "tetherusdte" | "tetherusdtt" | "tethereur" |
	"infinityeconomics" | "binancecoin" | "bitcoinsv" | "stasiseur" |
	"usdcoin" | "tron" | "binancesmartchain" | "binanceusd" |
	"coinspaid" | "dai" |
	"skrill" | "upi" | "papara" | "payfix" | "mefete" | "popypara" |
	"parazula" | "parolapara" | "payco" | "eft" | "fasthavale" |
	"sbp" | "sberpay" | "paytm" | "phonepe" | "walletm10" | "emanat" |
	"qrnspk" | "qrcode" | "ovo" | "grabpay" | "qris" | "cent" |
	"neteller" | "bkash_p2c" | "bkash_p2p" | "nagad_p2c" | "nagad_p2p" |
	"jazzcash" | "easypaisa" | "MTP" | "MTP_INTERNATIONAL" |
	"CARD_INTERNATIONAL" | "NSPK" |
	"sepa" | "banktransfer" | "imps" | "offramp" | "accountnumber" |
	"pesonet" | "instapay" | "ew" | "va" | "sofort" | "blik" |
	"volt" | "help2pay" | "ideal" | "SPEI" | "BANAMEX" | "BBVAMX" |
	"SANTANDER" | "HSBC_MX" | "BANORTE" | "AZTECA" | "SCOTIABANK" |
	"INBURSA" | "BAJIO" | "BANREGIO" | "BANCOPPEL" | "AFIRME" |
	"MIFEL" | "MULTIVA" | "INTERCAM" |
	"cardp2p" | "mobilecom" | "click" | "applepay" | "googlepay" | "other" |
	string

#CatalogAuthFlowType: "h2h" | "3ds" | "redirect" | "otp" | "qr" | "p2p" | "none"
