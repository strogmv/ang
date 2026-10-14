package normalizer

import (
	"strings"
	"testing"

	"cuelang.org/go/cue/cuecontext"
)

func TestTableName(t *testing.T) {
	cases := []struct{ entity, mode, want string }{
		// legacy (the default): lower + "s", whatever the word
		{"Company", "", "companys"},
		{"Company", TableNamesLegacy, "companys"},
		{"CompanyAddress", "", "companyaddresss"},
		{"TenderOfferTerms", "", "tenderoffertermss"},
		// plural
		{"Company", TableNamesPlural, "companies"},
		{"Country", TableNamesPlural, "countries"},
		{"Counterparty", TableNamesPlural, "counterparties"},
		{"CatalogShareQuantity", TableNamesPlural, "catalogsharequantities"},
		{"UserIdentity", TableNamesPlural, "useridentities"},
		{"APIKey", TableNamesPlural, "apikeys"},
		{"Survey", TableNamesPlural, "surveys"},
		{"CompanyAddress", TableNamesPlural, "companyaddresses"},
		{"OrderStatus", TableNamesPlural, "orderstatuses"},
		{"Tax", TableNamesPlural, "taxes"},
		{"Batch", TableNamesPlural, "batches"},
		{"Wish", TableNamesPlural, "wishes"},
		{"TenderOfferTerms", TableNamesPlural, "tenderofferterms"},
		{"Settings", TableNamesPlural, "settings"},
		{"User", TableNamesPlural, "users"},
		{"Tender", TableNamesPlural, "tenders"},
	}
	for _, c := range cases {
		if got := TableName(c.entity, "", c.mode); got != c.want {
			t.Errorf("TableName(%q, %q) = %q, want %q", c.entity, c.mode, got, c.want)
		}
	}
	if got := TableName("Person", "people", TableNamesPlural); got != "people" {
		t.Errorf("override: got %q", got)
	}
	if got := TableName("Person", "people", ""); got != "people" {
		t.Errorf("override in legacy mode: got %q", got)
	}
}

func TestEntityTableNameFallsBackToLegacy(t *testing.T) {
	if got := (Entity{Name: "Company"}).TableName(); got != "companys" {
		t.Fatalf("got %q", got)
	}
	if got := (Entity{Name: "Company", Table: "companies"}).TableName(); got != "companies" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractEntitiesResolvesTableNames(t *testing.T) {
	val := cuecontext.New().CompileString(`
Company: { id: string }
Person: {
	@db_table(name="people")
	id: string
}
`)
	if err := val.Err(); err != nil {
		t.Fatal(err)
	}
	for mode, want := range map[string]map[string]string{
		"":               {"Company": "companys", "Person": "people"},
		TableNamesPlural: {"Company": "companies", "Person": "people"},
	} {
		n := New()
		n.TableNames = mode
		entities, err := n.ExtractEntities(val)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entities {
			if e.Table != want[e.Name] {
				t.Errorf("mode %q: %s.Table = %q, want %q", mode, e.Name, e.Table, want[e.Name])
			}
		}
	}
}

func TestParseEntityDBTableRejectsBadNames(t *testing.T) {
	for _, src := range []string{
		`Customer: { @db_table(name="Customers"), id: string }`,
		`Customer: { @db_table(name="cust omers"), id: string }`,
		`Customer: { @db_table(people), id: string }`,
	} {
		_, err := parseEntityCUE(t, src)
		if err == nil || !strings.Contains(err.Error(), "@db_table") {
			t.Errorf("%s: err = %v", src, err)
		}
	}
}

func TestProjectTableNames(t *testing.T) {
	mode, err := ProjectTableNames([]TargetDef{{Name: "a", TableNames: "plural"}, {Name: "b", TableNames: "plural"}})
	if err != nil || mode != TableNamesPlural {
		t.Fatalf("mode = %q, err = %v", mode, err)
	}
	if mode, err := ProjectTableNames([]TargetDef{{Name: "a"}, {Name: "b", TableNames: "legacy"}}); err != nil || mode != "" {
		t.Fatalf("mode = %q, err = %v", mode, err)
	}
	if _, err := ProjectTableNames([]TargetDef{{Name: "a", TableNames: "plural"}, {Name: "b"}}); err == nil || !strings.Contains(err.Error(), "disagree") {
		t.Fatalf("err = %v", err)
	}
	if _, err := ProjectTableNames([]TargetDef{{Name: "a", TableNames: "plurals"}}); err == nil || !strings.Contains(err.Error(), "table_names must be") {
		t.Fatalf("err = %v", err)
	}
}

func TestExtractTargetReadsTableNames(t *testing.T) {
	val := cuecontext.New().CompileString(`state: target: {lang: "go", table_names: "plural"}`)
	td, err := New().ExtractTarget(val)
	if err != nil {
		t.Fatal(err)
	}
	if td.TableNames != TableNamesPlural {
		t.Fatalf("target = %#v", td)
	}
}

func TestQueueIndexNamesUseTableName(t *testing.T) {
	val := cuecontext.New().CompileString(`
ImportBatch: {
	@queue(handler="RunImport", key="companyId", unique="companyId")
	id:        string @db(type="UUID", primary_key=true)
	companyId: string @db(type="UUID")
}
`)
	if err := val.Err(); err != nil {
		t.Fatal(err)
	}
	n := New()
	n.TableNames = TableNamesPlural
	entities, err := n.ExtractEntities(val)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.AddQueueFields(entities); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, idx := range entities[0].Indexes {
		names = append(names, idx.Name)
	}
	got := strings.Join(names, " ")
	for _, want := range []string{"idx_importbatches_queue_ready", "idx_importbatches_queue_running", "idx_importbatches_queue_key", "uq_importbatches_queue_active"} {
		if !strings.Contains(got, want) {
			t.Errorf("indexes %s lack %s", got, want)
		}
	}
}
