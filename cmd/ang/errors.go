package main

import (
	"fmt"

	"github.com/strogmv/ang/compiler"
)

func formatStageFailure(prefix string, stage compiler.Stage, code, op string, err error) string {
	return fmt.Sprintf("%s: %v", prefix, compiler.WrapContractError(stage, code, op, err))
}

func printStageFailure(prefix string, stage compiler.Stage, code, op string, err error) {
	fmt.Println(formatStageFailure(prefix, stage, code, op, err))
}
