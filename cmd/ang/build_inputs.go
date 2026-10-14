package main

import (
	"fmt"
	"path/filepath"

	"cuelang.org/go/cue"
	"github.com/strogmv/ang/angir/normalizer"
	"github.com/strogmv/ang/angir/parser"
	"github.com/strogmv/ang/compiler"
	"github.com/strogmv/ang/compiler/schemavendor"
)

// buildInputs is what a build reads from the project's CUE before it
// compiles: the infrastructure bundle, RBAC (or policies), views, the project
// and its targets. A project without targets gets the default Go target.
type buildInputs struct {
	cueRoot           string
	tmplDir           string
	cfgDef            *normalizer.ConfigDef
	authDef           *normalizer.AuthDef
	sessionDef        *normalizer.SessionDef
	emailTemplates    []normalizer.EmailTemplateDef
	templatesCatalog  []normalizer.TemplateDef
	infraValues       map[string]any
	infraContextPatch normalizer.InfraContextPatch
	rbacDef           *normalizer.RBACDef
	views             []normalizer.ViewDef
	projectDef        *normalizer.ProjectDef
	targetDefs        []normalizer.TargetDef
	projectVal        cue.Value
}

// buildStageFailure is a failed build stage, reported by printStageFailure.
type buildStageFailure struct {
	stage compiler.Stage
	code  string
	op    string
	err   error
}

func stageFailure(stage compiler.Stage, code, op string, err error) *buildStageFailure {
	return &buildStageFailure{stage: stage, code: code, op: op, err: err}
}

// loadBuildInputs is the first stage of runBuild; a failure names the stage,
// code and operation for printStageFailure.
func loadBuildInputs(projectPath string) (buildInputs, *buildStageFailure) {
	p := parser.New()
	n := normalizer.New()

	var in buildInputs
	projCfg := loadProjectConfig(projectPath)
	cueRoot := projCfg.CueRoot
	in.cueRoot = cueRoot
	in.tmplDir = projCfg.TemplatesDir

	infraBundle, err := compiler.LoadInfraBundleWithRoot(projectPath, cueRoot)
	if err != nil {
		if ce, ok := err.(*compiler.ContractError); ok {
			return in, stageFailure(ce.Stage, ce.Code, ce.Op, ce.Err)
		}
		return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "load infrastructure bundle", err)
	}
	if infraBundle.Has {
		in.infraValues = infraBundle.Values
		in.cfgDef = infraBundle.Config
		in.authDef = infraBundle.Auth
		in.sessionDef = infraBundle.Session
		in.infraContextPatch = infraBundle.ContextPatch
		in.templatesCatalog = infraBundle.Templates
		in.emailTemplates = infraBundle.EmailTemplates
		in.templatesCatalog, err = resolveTemplates(projectPath, in.templatesCatalog)
		if err != nil {
			return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "resolve templates", err)
		}
		if len(in.templatesCatalog) > 0 {
			in.emailTemplates, err = templatesToEmail(in.templatesCatalog)
			if err != nil {
				return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "map templates to email templates", err)
			}
		} else {
			in.emailTemplates, err = resolveEmailTemplates(projectPath, in.emailTemplates)
			if err != nil {
				return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUEInfraConfigParse, "resolve email templates", err)
			}
		}
	}

	if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, cueRoot, "rbac")); err != nil {
		return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUERBACLoad, "load "+cueRoot+"/rbac", err)
	} else if ok {
		in.rbacDef, err = n.ExtractRBAC(val)
		if err != nil {
			return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUERBACParse, "extract rbac", err)
		}
	} else if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, cueRoot, "policies")); err != nil {
		return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUEPoliciesLoad, "load "+cueRoot+"/policies", err)
	} else if ok {
		in.rbacDef, err = n.ExtractRBAC(val)
		if err != nil {
			return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUEPoliciesParse, "extract policies as rbac", err)
		}
	}

	if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, cueRoot, "views")); err != nil {
		return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUEViewsLoad, "load "+cueRoot+"/views", err)
	} else if ok {
		in.views, err = n.ExtractViews(val)
		if err != nil {
			return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUEViewsParse, "extract views", err)
		}
	}

	if val, ok, err := compiler.LoadOptionalDomain(p, filepath.Join(projectPath, cueRoot, "project")); err != nil {
		return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUEProjectLoad, "load "+cueRoot+"/project", err)
	} else if ok {
		in.projectVal = val
		in.projectDef, err = n.ExtractProject(val)
		if err != nil {
			return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUEProjectParse, "extract project", err)
		}
		in.targetDefs, err = n.ExtractTargets(val)
		if err != nil {
			return in, stageFailure(compiler.StageCUE, compiler.ErrCodeCUETargetsParse, "extract targets", err)
		}
	}
	if len(in.targetDefs) == 0 {
		in.targetDefs = []normalizer.TargetDef{{
			Name:      "default",
			Lang:      "go",
			Framework: "chi",
			DB:        "postgres",
			Cache:     "redis",
			Queue:     "nats",
			Storage:   "s3",
		}}
	}

	// #Codegen comes from the schema the project imports: its own
	// cue/schema (the layout before vendoring), else the vendored one.
	for _, schemaDir := range []string{filepath.Join(projectPath, cueRoot, "schema"), schemavendor.Dir(projectPath)} {
		val, ok, err := compiler.LoadOptionalDomain(p, schemaDir)
		if err != nil || !ok || !val.LookupPath(cue.ParsePath("#Codegen")).Exists() {
			continue
		}
		if err := n.LoadCodegenConfig(val); err != nil {
			fmt.Printf("Warning: failed to load codegen config: %v\n", err)
		}
		break
	}
	return in, nil
}
