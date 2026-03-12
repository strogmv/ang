package emitter

import (
	"github.com/strogmv/ang-ir/normalizer"
	pyemitter "github.com/strogmv/ang/compiler/emitter/python"
)

func (e *Emitter) EmitPythonConfig(cfg *normalizer.ConfigDef) error {
	rt := pyemitter.Runtime{
		Funcs:        e.getSharedFuncMap(),
		ReadTemplate: ReadTemplateByPath,
	}
	return pyemitter.EmitConfig(e.OutputDir, rt, cfg)
}

func (e *Emitter) EmitPythonRBAC(rbac *normalizer.RBACDef) error {
	rt := pyemitter.Runtime{
		Funcs:        e.getSharedFuncMap(),
		ReadTemplate: ReadTemplateByPath,
	}
	return pyemitter.EmitRBAC(e.OutputDir, rt, rbac)
}

func (e *Emitter) EmitPythonAuthStores(auth *normalizer.AuthDef) error {
	rt := pyemitter.Runtime{
		Funcs:        e.getSharedFuncMap(),
		ReadTemplate: ReadTemplateByPath,
	}
	return pyemitter.EmitAuthStores(e.OutputDir, rt, auth)
}
