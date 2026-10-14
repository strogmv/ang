package sqltext

import (
	"reflect"
	"testing"
)

func TestPlaceholdersSkipStringsCommentsAndDollarQuotes(t *testing.T) {
	sql := `
		-- $9 in a line comment
		SELECT '$8', E'it''s \' $7', "$6", $tag$ $5 $tag$, $$ $4 $$ /* $3 /* nested $2 */ */
		FROM t WHERE a = $1 AND b = ANY($2::uuid[]) AND c = $1 LIMIT $10`
	got, err := Placeholders(sql)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{1, 2, 10}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Placeholders = %v, want %v", got, want)
	}
}

func TestTokensRejectUnterminated(t *testing.T) {
	for _, sql := range []string{"SELECT 'x", `SELECT "x`, "SELECT /* x", "SELECT $a$ x"} {
		if _, err := Tokens(sql); err == nil {
			t.Errorf("Tokens(%q): want an error", sql)
		}
	}
}

func names(cols []Column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Name
	}
	return out
}

func TestResultColumns(t *testing.T) {
	cases := []struct {
		name string
		sql  string
		want []string
	}{
		{"plain", "SELECT id::text, t.title, COALESCE(x, '') AS note, count(*) FROM t", []string{"id", "title", "note", ""}},
		{"distinct on", "SELECT DISTINCT ON (a, b) a, b FROM t", []string{"a", "b"}},
		{"cte", "WITH x AS (SELECT a, b, c FROM t), y AS (SELECT 1) SELECT x.a FROM x", []string{"a"}},
		{"cte update returning", "WITH c AS (UPDATE t SET s = 1 RETURNING 1, 2) SELECT count(*) FROM c", []string{""}},
		{"returning", "UPDATE t SET a = $1 WHERE id = $2 RETURNING id::text, a", []string{"id", "a"}},
		{"insert select returning", "INSERT INTO t (a, b) SELECT a, b FROM u ON CONFLICT DO NOTHING RETURNING a", []string{"a"}},
		{"case and arrays", "SELECT CASE WHEN a IN (1, 2) THEN 1 ELSE 0 END, ARRAY[1, 2], x FROM t", []string{"", "", "x"}},
		{"is distinct from", "SELECT a IS DISTINCT FROM b, c FROM t", []string{"", "c"}},
		{"implicit alias", "SELECT t.a::text a_text, b total FROM t", []string{"a_text", "total"}},
		{"multiword cast", "SELECT a::timestamp with time zone, b::double precision FROM t", []string{"a", "b"}},
		{"union", "SELECT a, b FROM t UNION ALL SELECT c, d FROM u", []string{"a", "b"}},
		{"to_char", `SELECT to_char(l.effectiveat AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), l.source FROM l`, []string{"", "source"}},
	}
	for _, tc := range cases {
		cols, ok := ResultColumns(tc.sql)
		if !ok {
			t.Errorf("%s: ResultColumns not ok", tc.name)
			continue
		}
		if got := names(cols); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: names = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestResultColumnsStarAndCast(t *testing.T) {
	cols, ok := ResultColumns("SELECT p.*, x.id::text FROM p JOIN x ON true")
	if !ok || len(cols) != 2 {
		t.Fatalf("cols = %+v ok=%v", cols, ok)
	}
	if !cols[0].Star || cols[0].Qualifier != "p" || cols[0].Text != "p.*" {
		t.Fatalf("first column = %+v", cols[0])
	}
	if !cols[1].Plain || cols[1].Cast != "text" {
		t.Fatalf("second column = %+v", cols[1])
	}
}

func TestResultColumnsUnknown(t *testing.T) {
	for _, sql := range []string{
		"UPDATE t SET a = 1",
		"(SELECT a FROM t) UNION (SELECT b FROM u)",
		"VALUES (1, 2)",
		"SELECT 'unterminated",
	} {
		if cols, ok := ResultColumns(sql); ok {
			t.Errorf("ResultColumns(%q) = %+v, want unknown", sql, cols)
		}
	}
}
