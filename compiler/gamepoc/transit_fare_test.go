package gamepoc

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/strogmv/ang/compiler/inventory"
	inv "github.com/strogmv/ang/compiler/inventory/runtime"
)

func TestTransitFareCompilerRejectsMissingAccountsAndHiddenPrices(t *testing.T) {
	base := Compiled{Inventory: inv.Model{Currencies: map[string]inv.Currency{"CRD": {Code: "CRD", Carrier: "account", Scale: 100}}, Accounts: map[string]inv.Account{"receiver": {ID: "receiver", Currency: "CRD", Owner: "institution"}}}, Localization: LocalizationCatalog{Messages: []LocalizationMessage{{Key: "transit.paid", Texts: map[string]string{"ru": "Поезд · {amount} {currency}", "en": "Train · {amount} {currency}"}}}}}
	edge := map[string]any{"id": "rail", "mode": "rail", "name_key": "transit.paid", "fare": map[string]any{"currency": "CRD", "amount_minor": 100, "receiver_account": "receiver"}}
	check := func(e map[string]any, texts LocalizationCatalog) error {
		c := base
		c.Localization = texts
		data, _ := json.Marshal(map[string]any{"edges": []any{e}})
		c.Life = data
		return validateTransitFares(c)
	}
	if err := check(edge, base.Localization); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"unknown_receiver", "wrong_currency", "free_mode", "unnamed", "price_hidden"} {
		t.Run(kind, func(t *testing.T) {
			data, _ := json.Marshal(edge)
			var e map[string]any
			json.Unmarshal(data, &e)
			fare := e["fare"].(map[string]any)
			texts := base.Localization
			switch kind {
			case "unknown_receiver":
				fare["receiver_account"] = "missing"
			case "wrong_currency":
				fare["currency"] = "OTHER"
			case "free_mode":
				e["mode"] = "walk"
			case "unnamed":
				delete(e, "name_key")
			case "price_hidden":
				texts = LocalizationCatalog{Messages: []LocalizationMessage{{Key: "transit.paid", Texts: map[string]string{"en": "Train"}}}}
			}
			if check(e, texts) == nil {
				t.Fatal("invalid paid-route metadata accepted")
			}
		})
	}
}

func TestTransitFareAtomicPaymentNative(t *testing.T) {
	emitted, err := inventory.Render()
	if err != nil {
		t.Fatal(err)
	}
	template := hudTemplate(t, "transit_fare.go.tmpl")
	start := strings.Index(template, "func payLifeFare(")
	if start < 0 {
		t.Fatal("payment helper missing")
	}
	files := map[string]string{
		"go.mod":                    "module angelsdemons/server\n\ngo 1.23\n",
		"internal/behavior/fare.go": "package behavior\ntype LifeFare struct{Currency string;AmountMinor int64;ReceiverAccount string}\n",
		"internal/poc/payment.go":   "package poc\nimport(\"sort\";\"angelsdemons/server/internal/behavior\";inv \"angelsdemons/server/internal/inventory\")\n" + template[start:],
		"internal/poc/payment_test.go": `package poc
 import("reflect";"testing";"angelsdemons/server/internal/behavior";inv "angelsdemons/server/internal/inventory")
 func fixture(t *testing.T)(inv.Model,inv.State){m:=inv.Model{Version:1,MaxDepth:8,MaxItems:100,Reservation:inv.ReservationPolicy{MaxTicks:600,MaxPerActor:16,MaxTotal:128},Actors:map[string]bool{"hero":true},Owners:map[string]bool{"hero":true,"institution":true},Levels:map[string]inv.Cell{"hall":{X:5,Y:5}},Definitions:map[string]inv.Definition{},Currencies:map[string]inv.Currency{"CRD":{Code:"CRD",Carrier:"account",Scale:100}},Accounts:map[string]inv.Account{"payer":{ID:"payer",Owner:"hero",Currency:"CRD",Balance:1000},"transport":{ID:"transport",Owner:"institution",Currency:"CRD"}},Stores:map[string]inv.Store{inv.ActorStore("hero"):{ID:inv.ActorStore("hero"),Kind:"actor",Owner:"hero",Anchor:inv.Anchor{Kind:"actor",Actor:"hero"},Capacity:inv.Capacity{Mass:1000,Volume:1000,Stacks:10}}}};s,err:=inv.New(m,777);if err!=nil{t.Fatal(err)};return m,s}
 func TestActualPaymentIsConservedAndSequenced(t *testing.T){m,s:=fixture(t);fare:=&behavior.LifeFare{Currency:"CRD",AmountMinor:100,ReceiverAccount:"transport"};if !payLifeFare(&s,m,"hero",fare,"boarding:1"){t.Fatal("valid fare rejected")};if s.Accounts["payer"].Balance!=900||s.Accounts["transport"].Balance!=100||s.Cursors["hero"].Sequence!=1||s.Accounts["payer"].Revision!=1||s.Accounts["transport"].Revision!=1{t.Fatal("payment bypassed atomic inventory accounting")};if err:=s.Validate(m);err!=nil{t.Fatal(err)}}
 func TestRejectedPaymentLeavesEntireInventoryUntouched(t *testing.T){for _,kind:=range []string{"insufficient","missing_receiver","wrong_currency","own_receiver","invalid_amount"}{t.Run(kind,func(t *testing.T){m,s:=fixture(t);fare:=&behavior.LifeFare{Currency:"CRD",AmountMinor:100,ReceiverAccount:"transport"};switch kind{case "insufficient":a:=s.Accounts["payer"];a.Balance=99;s.Accounts["payer"]=a;case "missing_receiver":fare.ReceiverAccount="missing";case "wrong_currency":fare.Currency="OTHER";case "own_receiver":fare.ReceiverAccount="payer";case "invalid_amount":fare.AmountMinor=0};before:=s.Clone();if payLifeFare(&s,m,"hero",fare,"boarding:failed")||!reflect.DeepEqual(before,s){t.Fatal("rejected fare changed items, money, revisions or cursors")}})}}
 `,
	}
	for name, data := range emitted {
		if !strings.HasSuffix(name, "_test.go") {
			files["internal/inventory/"+name] = string(data)
		}
	}
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "./internal/poc", "-count=1", "-v")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual inventory payment template: %v\n%s", err, out)
	}
}
