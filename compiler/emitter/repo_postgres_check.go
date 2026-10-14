package emitter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/sqltext"
)

// FinderProblem is a finder whose generated Postgres method could not work:
// its SQL and its Go disagree on the parameters or on the result columns.
type FinderProblem struct {
	Source  string // file:line of the finder
	Entity  string
	Finder  string
	Message string
}

func (p FinderProblem) String() string {
	where := p.Source
	if where == "" {
		where = "cue/repo"
	}
	return fmt.Sprintf("%s: finder %s.%s: %s", where, p.Entity, p.Finder, p.Message)
}

// ValidatePostgresFinders plans every Postgres repository the way
// EmitPostgresRepo does and checks each hand-written sql: against the
// generated method: $n placeholders against the arguments, result columns
// against the scan, and kind: "upsert" against the entity's unique indexes.
func ValidatePostgresFinders(repos []normalizer.Repository, entities []normalizer.Entity) []FinderProblem {
	entMap := make(map[string]normalizer.Entity, len(entities))
	for _, ent := range entities {
		entMap[ent.Name] = ent
	}
	var problems []FinderProblem
	for _, repo := range repos {
		ent, ok := entMap[repo.Entity]
		if !ok || !hasSQLTable(ent) {
			continue
		}
		dbFields, allSelectCols := pgEntityColumns(ent)
		finders, _ := buildPgFinders(repo, ent, entMap, dbFields, allSelectCols)
		for _, fo := range finders {
			problems = append(problems, checkPgFinderSQL(repo.Entity, fo)...)
		}
		_, upsertProblems := buildPgUpserts(repo, ent, dbFields)
		problems = append(problems, upsertProblems...)
	}
	sort.SliceStable(problems, func(i, j int) bool { return problems[i].Source < problems[j].Source })
	return problems
}

// pgEntityColumns lists the columns EmitPostgresRepo stores and selects.
func pgEntityColumns(ent normalizer.Entity) ([]normalizer.Field, []string) {
	var dbFields []normalizer.Field
	var cols []string
	for _, f := range ent.Fields {
		if f.SkipDomain {
			continue
		}
		dbFields = append(dbFields, f)
		cols = append(cols, scanSelectColumnExpr(f))
	}
	return dbFields, cols
}

// finderArgNames lists the arguments the generated method passes to its
// query, in $n order.
func finderArgNames(f normalizer.RepositoryFinder) []string {
	var out []string
	for _, w := range f.Where {
		out = append(out, w.Param)
	}
	for _, p := range f.Params {
		out = append(out, p.Name)
	}
	if normalizer.FinderPaged(f) {
		out = append(out, "limit", "offset")
	}
	return out
}

func checkPgFinderSQL(entity string, fo pgFinder) []FinderProblem {
	f := fo.RepositoryFinder
	if strings.TrimSpace(f.CustomSQL) == "" {
		return nil
	}
	problem := func(format string, args ...any) FinderProblem {
		return FinderProblem{
			Source:  f.Source,
			Entity:  entity,
			Finder:  ExportName(f.Name),
			Message: fmt.Sprintf(format, args...),
		}
	}
	var out []FinderProblem
	if fo.selectAutoErr != nil {
		out = append(out, problem("%v", fo.selectAutoErr))
		return out
	}

	// Parameters: the SQL must use exactly $1..$n for the n arguments.
	args := finderArgNames(f)
	used, err := sqltext.Placeholders(f.CustomSQL)
	if err != nil {
		return append(out, problem("sql: %v", err))
	}
	usedSet := map[int]bool{}
	for _, n := range used {
		usedSet[n] = true
	}
	describe := func() string {
		if len(args) == 0 {
			return "the method passes no arguments"
		}
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprintf("$%d=%s", i+1, a)
		}
		return "the method passes " + strings.Join(parts, ", ")
	}
	if len(used) > 0 && used[len(used)-1] > len(args) {
		var extra []string
		for _, n := range used {
			if n > len(args) {
				extra = append(extra, fmt.Sprintf("$%d", n))
			}
		}
		hint := "declare the parameter in params: (or paged: true for limit/offset)"
		out = append(out, problem("sql uses %s, but %s; %s", strings.Join(extra, ", "), describe(), hint))
	}
	var missing []string
	for i := 1; i <= len(args); i++ {
		if !usedSet[i] {
			missing = append(missing, fmt.Sprintf("$%d (%s)", i, args[i-1]))
		}
	}
	if len(missing) > 0 {
		out = append(out, problem("sql never uses %s; %s, and Postgres rejects an argument it does not use", strings.Join(missing, ", "), describe()))
	}

	// Result columns.
	cols, ok := sqltext.ResultColumns(f.CustomSQL)
	if !ok {
		return out
	}
	for _, c := range cols {
		if c.Star {
			// SELECT * returns the table's columns in table order; the scan
			// follows the entity. select: "auto" writes the list instead.
			return out
		}
	}
	var expected []normalizer.Field
	switch {
	case fo.SelectEntity || fo.SelectCustomEntity:
		for _, v := range fo.ScanPlan.Variables {
			for _, sf := range fo.SelectFields {
				if sf.Name == v.Name {
					expected = append(expected, sf)
					break
				}
			}
		}
		if len(expected) != len(fo.ScanPlan.Variables) {
			return out
		}
	case len(fo.SelectFields) > 0:
		// A scalar result: the method scans one column.
		if len(cols) != 1 {
			out = append(out, problem("sql returns %d columns, but the method returns %s and scans exactly one", len(cols), fo.ReturnType))
		}
		return out
	default:
		return out
	}
	expectedNames := make([]string, len(expected))
	for i, fld := range expected {
		expectedNames[i] = DBName(fld.Name)
	}
	if len(cols) != len(expected) {
		out = append(out, problem("sql returns %d columns, but the method scans %d into %s, in this order: %s; fix the select list or use select: \"auto\"",
			len(cols), len(expected), scanTargetName(entity, fo), strings.Join(expectedNames, ", ")))
		return out
	}
	position := map[string]int{}
	for i, name := range expectedNames {
		position[name] = i
	}
	for i, c := range cols {
		name := normalizeColumnName(c.Name)
		if name == "" || name == expectedNames[i] {
			if name == expectedNames[i] && c.Plain && c.Cast == "" {
				fld := expected[i]
				if fld.Type == "string" && !fld.IsOptional && pgTypeNeedsTextCast(fld.DB.Type) {
					out = append(out, problem("column %d (%s) is %s, which pgx cannot scan into a Go string; write %s::text or use select: \"auto\"",
						i+1, c.Text, strings.ToLower(fld.DB.Type), c.Text))
				}
			}
			continue
		}
		if j, ok := position[name]; ok {
			out = append(out, problem("column %d of the sql is %s, but the method scans %s there and %s at position %d; the scan follows the entity's field order: %s",
				i+1, name, expectedNames[i], name, j+1, strings.Join(expectedNames, ", ")))
			return out
		}
	}
	return out
}

func scanTargetName(entity string, fo pgFinder) string {
	if fo.SelectCustomEntity {
		return "domain." + ExportName(fo.CustomEntityName)
	}
	return "domain." + ExportName(entity)
}

func normalizeColumnName(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "_", ""))
}

// pgTypeNeedsTextCast reports whether pgx v5 refuses to scan a column of
// this type into a *string (binary format): timestamps, dates, times,
// booleans, network addresses. uuid, numeric, json and the text types scan.
func pgTypeNeedsTextCast(dbType string) bool {
	t := strings.ToUpper(strings.TrimSpace(dbType))
	for _, prefix := range []string{"TIMESTAMP", "DATE", "TIME", "BOOL", "INET", "CIDR", "INTERVAL"} {
		if strings.HasPrefix(t, prefix) {
			return true
		}
	}
	return false
}

// expandSelectAuto replaces the * (or alias.*) of a select: "auto" finder's
// SQL with the columns its method scans, in scan order, each cast the way
// the generated finders cast them (a non-text string column ::text).
func expandSelectAuto(fo pgFinder) (string, error) {
	sql := fo.CustomSQL
	if !(fo.SelectEntity || fo.SelectCustomEntity) || len(fo.ScanPlan.Columns) == 0 {
		return sql, fmt.Errorf(`select: "auto" needs a finder that returns the entity or a return_type with scan_fields; this one returns %s`, fo.ReturnType)
	}
	cols, ok := sqltext.ResultColumns(sql)
	if !ok {
		return sql, fmt.Errorf(`select: "auto": cannot find the select list of the sql`)
	}
	star := -1
	for i, c := range cols {
		if c.Star {
			if star >= 0 {
				return sql, fmt.Errorf(`select: "auto": the select list has more than one *`)
			}
			star = i
		}
	}
	if star < 0 || len(cols) != 1 {
		return sql, fmt.Errorf(`select: "auto": the select list must be exactly * or alias.* (ANG writes the columns); it is %q`, joinColumnText(cols))
	}
	c := cols[star]
	list := make([]string, len(fo.ScanPlan.Columns))
	for i, col := range fo.ScanPlan.Columns {
		if c.Qualifier != "" {
			list[i] = sqltext.QuoteIdent(c.Qualifier) + "." + col
		} else {
			list[i] = col
		}
	}
	return sql[:c.Start] + strings.Join(list, ", ") + sql[c.End:], nil
}

func joinColumnText(cols []sqltext.Column) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = c.Text
	}
	return strings.Join(parts, ", ")
}

// pgUpsert is one kind: "upsert" finder: a batch INSERT … ON CONFLICT on a
// unique index other than (or as well as) the primary key.
type pgUpsert struct {
	Name           string
	ConflictTarget string // "(companyid, sku)" plus " WHERE …" for a partial index
	UpdateSet      string
	Source         string
}

// buildPgUpserts plans the kind: "upsert" finders of a repository. on_conflict
// must name the fields of a unique index of the entity (or the id), because
// Postgres refuses ON CONFLICT on columns without one at run time.
func buildPgUpserts(repo normalizer.Repository, ent normalizer.Entity, dbFields []normalizer.Field) ([]pgUpsert, []FinderProblem) {
	var out []pgUpsert
	var problems []FinderProblem
	taken := map[string]bool{"Save": true, "SaveBatch": true, "FindByID": true, "Delete": true, "Insert": true, "Update": true, "LockByID": true, "ListAll": true}
	for _, f := range repo.Finders {
		if f.Kind != "upsert" {
			taken[ExportName(f.Name)] = true
		}
	}
	for _, f := range repo.Finders {
		if f.Kind != "upsert" {
			continue
		}
		problem := func(format string, args ...any) {
			problems = append(problems, FinderProblem{Source: f.Source, Entity: repo.Entity, Finder: ExportName(f.Name), Message: fmt.Sprintf(format, args...)})
		}
		if taken[ExportName(f.Name)] {
			problem("the repository already has a method %s; give the upsert another name", ExportName(f.Name))
			continue
		}
		taken[ExportName(f.Name)] = true
		var conflictCols []string
		conflictSet := map[string]bool{}
		for _, name := range f.OnConflict {
			found := false
			for _, fld := range dbFields {
				if strings.EqualFold(fld.Name, name) || DBName(fld.Name) == DBName(name) {
					conflictCols = append(conflictCols, DBName(fld.Name))
					conflictSet[DBName(fld.Name)] = true
					found = true
					break
				}
			}
			if !found {
				problem("on_conflict: %s is not a field of %s", name, repo.Entity)
			}
		}
		if len(conflictCols) != len(f.OnConflict) {
			continue
		}
		where, ok := matchUniqueIndex(ent, conflictSet)
		if !ok {
			problem("on_conflict: (%s) is not the column list of a unique index of %s; add indexes: [{fields: [%s], unique: true}] to the entity",
				strings.Join(conflictCols, ", "), repo.Entity, quotedList(f.OnConflict))
			continue
		}
		target := "(" + strings.Join(conflictCols, ", ") + ")"
		if where != "" {
			target += " WHERE " + where
		}
		var sets []string
		for _, fld := range dbFields {
			col := DBName(fld.Name)
			if col == "id" || conflictSet[col] {
				continue
			}
			sets = append(sets, fmt.Sprintf("%s = EXCLUDED.%s", col, col))
		}
		u := pgUpsert{Name: ExportName(f.Name), ConflictTarget: target, UpdateSet: strings.Join(sets, ", "), Source: f.Source}
		out = append(out, u)
	}
	return out, problems
}

// matchUniqueIndex finds a unique, non-expression index whose columns are
// exactly cols (the id alone is the primary key). where is the predicate of
// a partial index, which ON CONFLICT must repeat to infer it.
func matchUniqueIndex(ent normalizer.Entity, cols map[string]bool) (string, bool) {
	if len(cols) == 1 && cols["id"] {
		return "", true
	}
	for _, idx := range ent.Indexes {
		if !idx.Unique || idx.Expr != "" || len(idx.Fields) != len(cols) {
			continue
		}
		all := true
		for _, name := range idx.Fields {
			if !cols[DBName(name)] {
				all = false
				break
			}
		}
		if all {
			return idx.Where, true
		}
	}
	return "", false
}

func quotedList(items []string) string {
	parts := make([]string, len(items))
	for i, s := range items {
		parts[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(parts, ", ")
}

func init() {
	compiler.RegisterRepoFinderCheck(func(repos []normalizer.Repository, entities []normalizer.Entity) []string {
		var out []string
		for _, p := range ValidatePostgresFinders(repos, entities) {
			out = append(out, p.String())
		}
		return out
	})
}
