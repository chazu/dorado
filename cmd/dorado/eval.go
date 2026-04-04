package main

import (
	"fmt"
	"strings"

	"github.com/chazu/maggie/vm"
)

// evalExpression compiles and executes a Maggie expression, returning
// the result as a printString and any error.
func evalExpression(vmInst *vm.VM, source string) (result vm.Value, printStr string, err error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return vm.Nil, "", nil
	}

	method, compileErr := vmInst.CompileExpression(source)
	if compileErr != nil {
		return vm.Nil, "", fmt.Errorf("compile: %w", compileErr)
	}

	result, err = vmInst.ExecuteSafe(method, vm.Nil, nil)
	if err != nil {
		return vm.Nil, "", err
	}

	// Get printString of result
	printStr = valuePrintString(vmInst, result)
	return result, printStr, nil
}

// valuePrintString sends printString to a value and returns the Go string.
func valuePrintString(vmInst *vm.VM, val vm.Value) string {
	defer func() {
		if r := recover(); r != nil {
			// If printString itself fails, fall back
		}
	}()

	result := vmInst.Send(val, "printString", nil)
	if vm.IsStringValue(result) {
		return vmInst.Registry().GetStringContent(result)
	}
	// Fallback for symbols
	if result.IsSymbol() {
		return vmInst.Symbols.Name(result.SymbolID())
	}
	return fmt.Sprintf("%v", result)
}
