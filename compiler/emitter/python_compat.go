package emitter

import pyemitter "github.com/strogmv/ang/compiler/emitter/python"

// Compatibility wrappers kept in emitter package for existing tests and
// transition code paths while Python runtime lives in emitter/python.
func mergePythonCustomBlocks(generated, existing string) string {
	return pyemitter.MergeCustomBlocks(generated, existing)
}
