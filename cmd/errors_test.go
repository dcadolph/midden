package cmd

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorCode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		In       error
		WantCode int
		WantOK   bool
	}{{ // Test 0: Nil error is unmapped.
		In: nil, WantCode: 0, WantOK: false,
	}, { // Test 1: Plain errors are unmapped.
		In: errors.New("boom"), WantCode: 0, WantOK: false,
	}, { // Test 2: ErrVault maps to the vault exit code.
		In: ErrVault, WantCode: ExitVault, WantOK: true,
	}, { // Test 3: ErrNotFound maps to the not-found exit code.
		In: ErrNotFound, WantCode: ExitNotFound, WantOK: true,
	}, { // Test 4: ErrEditor maps to the editor exit code.
		In: ErrEditor, WantCode: ExitEditor, WantOK: true,
	}, { // Test 5: ErrLLM maps to the llm exit code.
		In: ErrLLM, WantCode: ExitLLM, WantOK: true,
	}, { // Test 6: ErrGit maps to the git exit code.
		In: ErrGit, WantCode: ExitGit, WantOK: true,
	}, { // Test 7: Joined errors keep their sentinel mapping.
		In: errors.Join(ErrLLM, errors.New("provider down")), WantCode: ExitLLM, WantOK: true,
	}, { // Test 8: fmt-wrapped errors keep their sentinel mapping.
		In: fmt.Errorf("sync: %w", ErrGit), WantCode: ExitGit, WantOK: true,
	}, { // Test 9: ErrVault wins when joined with ErrNotFound.
		In: errors.Join(ErrVault, ErrNotFound), WantCode: ExitVault, WantOK: true,
	}}
	for testNum, test := range tests {
		t.Run(fmt.Sprintf("test %d", testNum), func(t *testing.T) {
			t.Parallel()
			code, ok := errorCode(test.In)
			if code != test.WantCode || ok != test.WantOK {
				t.Errorf("errorCode(%v) = (%d, %t), want (%d, %t)",
					test.In, code, ok, test.WantCode, test.WantOK)
			}
		})
	}
}
