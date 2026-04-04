package main

import (
	"fmt"
	"strings"

	"github.com/chazu/dorado/go/display"
	"github.com/chazu/maggie/vm"
)

// openInspector creates an Inspector window for the given Maggie value.
func openInspector(vmInst *vm.VM, val vm.Value) {
	title := "Inspector: " + shortDescription(vmInst, val)
	w := display.NewWindow(200, 150, 500, 350, title)

	// Build inspector content
	content := buildInspectorText(vmInst, val)
	w.SetEditor(content)

	app.wm.AddWindow(w)
}

// buildInspectorText creates a text representation of an object's state
// suitable for display in an inspector window.
func buildInspectorText(vmInst *vm.VM, val vm.Value) string {
	var sb strings.Builder

	className := valueClassName(vmInst, val)
	printStr := valuePrintString(vmInst, val)

	sb.WriteString(fmt.Sprintf("self: %s  (%s)\n", printStr, className))
	sb.WriteString(strings.Repeat("─", 40))
	sb.WriteString("\n")

	// For objects with instance variables, show them
	if val.IsObject() {
		obj := vm.ObjectFromValue(val)
		if obj != nil {
			class := vmInst.GetClassFromValue(val)
			if class != nil {
				ivars := class.AllInstVarNames()
				for i, name := range ivars {
					if i < obj.NumSlots() {
						slotVal := obj.GetSlot(i)
						slotStr := valuePrintString(vmInst, slotVal)
						sb.WriteString(fmt.Sprintf("%-20s %s\n", name+":", slotStr))
					}
				}
				if len(ivars) == 0 {
					sb.WriteString("(no instance variables)\n")
				}
			}
		}
	} else if val.IsSmallInt() {
		sb.WriteString(fmt.Sprintf("%-20s %d\n", "value:", val.SmallInt()))
		sb.WriteString(fmt.Sprintf("%-20s 0x%X\n", "hex:", val.SmallInt()))
		// Show some useful derived values
		sb.WriteString(fmt.Sprintf("%-20s %s\n", "even:", boolStr(val.SmallInt()%2 == 0)))
	} else if vm.IsStringValue(val) {
		str := vmInst.Registry().GetStringContent(val)
		sb.WriteString(fmt.Sprintf("%-20s %d\n", "size:", len([]rune(str))))
		// Show first few characters
		if len(str) > 100 {
			sb.WriteString(fmt.Sprintf("%-20s '%s...'\n", "contents:", str[:100]))
		} else {
			sb.WriteString(fmt.Sprintf("%-20s '%s'\n", "contents:", str))
		}
	} else if val == vm.True || val == vm.False {
		sb.WriteString(fmt.Sprintf("%-20s %s\n", "value:", printStr))
	} else if val == vm.Nil {
		sb.WriteString("(nil)\n")
	} else if val.IsSymbol() {
		name := vmInst.Symbols.Name(val.SymbolID())
		sb.WriteString(fmt.Sprintf("%-20s #%s\n", "name:", name))
		sb.WriteString(fmt.Sprintf("%-20s %d\n", "symbolID:", val.SymbolID()))
	}

	return sb.String()
}

func valueClassName(vmInst *vm.VM, val vm.Value) string {
	defer func() { recover() }()
	result := vmInst.Send(val, "class", nil)
	nameResult := vmInst.Send(result, "name", nil)
	if nameResult.IsSymbol() {
		return vmInst.Symbols.Name(nameResult.SymbolID())
	}
	if vm.IsStringValue(nameResult) {
		return vmInst.Registry().GetStringContent(nameResult)
	}
	return "?"
}

func shortDescription(vmInst *vm.VM, val vm.Value) string {
	ps := valuePrintString(vmInst, val)
	if len(ps) > 40 {
		return ps[:40] + "..."
	}
	return ps
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
