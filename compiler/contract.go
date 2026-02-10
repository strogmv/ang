package compiler

import "fmt"

// Stage defines a formal compiler pipeline stage.
type Stage string

const (
	StageCUE          Stage = "CUE"
	StageIR           Stage = "IR"
	StageTransformers Stage = "TRANSFORMERS"
	StageEmitters     Stage = "EMITTERS"
)

// ContractError is a typed pipeline error with stage and stable code.
type ContractError struct {
	Stage Stage
	Code  string
	Op    string
	Err   error
}

func (e *ContractError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Op == "" {
		return fmt.Sprintf("[%s:%s] %v", e.Stage, e.Code, e.Err)
	}
	return fmt.Sprintf("[%s:%s] %s: %v", e.Stage, e.Code, e.Op, e.Err)
}

func (e *ContractError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// WrapContractError wraps err into ContractError and keeps the cause chain.
func WrapContractError(stage Stage, code, op string, err error) error {
	if err == nil {
		return nil
	}
	return &ContractError{
		Stage: stage,
		Code:  code,
		Op:    op,
		Err:   err,
	}
}
