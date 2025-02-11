package native

import (
	"bytes"
	"encoding/json"
	"github.com/FusionFoundation/efsn/v5/accounts/abi"
	"github.com/FusionFoundation/efsn/v5/crypto"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/FusionFoundation/efsn/v5/common"
	"github.com/FusionFoundation/efsn/v5/core/vm"
	"github.com/FusionFoundation/efsn/v5/eth/tracers"
)

func init() {
	register("returnMsgTracer", newReturnMsgTracer)
}

var revertSelector = crypto.Keccak256([]byte("Error(string)"))[:4]

// returnMsgTracer is a go implementation of the Tracer interface which
// track the error message return by the contract.
type returnMsgTracer struct {
	env       *vm.EVM
	returnMsg string
	interrupt uint32 // Atomic flag to signal execution interruption
	reason    error  // Textual reason for the interruption
}

// newNoopTracer returns a new noop tracer.
func newReturnMsgTracer() tracers.Tracer {
	return &returnMsgTracer{}
}

// CaptureStart implements the EVMLogger interface to initialize the tracing operation.
func (t *returnMsgTracer) CaptureStart(env *vm.EVM, _ common.Address, _ common.Address, _ bool, _ []byte, _ uint64, _ *big.Int) {
	t.env = env
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (t *returnMsgTracer) CaptureEnd(output []byte, _ uint64, _ time.Duration, err error) {
	if err != nil {
		if err == vm.ErrExecutionReverted && len(output) > 4 && bytes.Equal(output[:4], revertSelector) {
			returnMsg, _ := abi.UnpackRevert(output)
			t.returnMsg = err.Error() + ": " + returnMsg
		} else {
			t.returnMsg = err.Error()
		}
	}
}

// CaptureState implements the EVMLogger interface to trace a single step of VM execution.
func (t *returnMsgTracer) CaptureState(_ uint64, _ vm.OpCode, _, _ uint64, _ *vm.ScopeContext, _ []byte, _ int, _ error) {
}

// CaptureFault implements the EVMLogger interface to trace an execution fault.
func (t *returnMsgTracer) CaptureFault(_ uint64, _ vm.OpCode, _, _ uint64, _ *vm.ScopeContext, _ int, _ error) {
}

// CaptureEnter is called when EVM enters a new scope (via call, create or selfdestruct).
func (t *returnMsgTracer) CaptureEnter(_ vm.OpCode, _ common.Address, _ common.Address, _ []byte, _ uint64, _ *big.Int) {
	// Skip if tracing was interrupted
	if atomic.LoadUint32(&t.interrupt) > 0 {
		t.env.Cancel()
		return
	}
}

// CaptureExit is called when EVM exits a scope, even if the scope didn't
// execute any code.
func (t *returnMsgTracer) CaptureExit(_ []byte, _ uint64, _ error) {
}

// GetResult returns an error message json object.
func (t *returnMsgTracer) GetResult() (json.RawMessage, error) {
	res, err := json.Marshal(t.returnMsg)
	if err != nil {
		return nil, err
	}
	return res, t.reason
}

// Stop terminates execution of the tracer at the first opportune moment.
func (t *returnMsgTracer) Stop(err error) {
	t.reason = err
	atomic.StoreUint32(&t.interrupt, 1)
}
