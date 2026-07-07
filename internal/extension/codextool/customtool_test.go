package codextool

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNamespacedToolNamePreservesExistingSeparator(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		namespace string
		tool      string
		want      string
	}{
		{name: "plain namespace", namespace: "mcp__catalyst_search", tool: "search", want: "mcp__catalyst_search_search"},
		{name: "codex namespace", namespace: "mcp__catalyst_search__", tool: "read_url", want: "mcp__catalyst_search__read_url"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if got := NamespacedToolName(testCase.namespace, testCase.tool); got != testCase.want {
				t.Fatalf("NamespacedToolName(%q, %q) = %q, want %q", testCase.namespace, testCase.tool, got, testCase.want)
			}
		})
	}
}

func TestOutputItemFromBlockRestoresUniqueBareFunctionAlias(t *testing.T) {
	toolMap := ToolMap{
		"mcp__catalyst_search__read_url": {
			Kind:       ToolFunction,
			OpenAIName: "read_url",
			Namespace:  "mcp__catalyst_search__",
		},
	}

	itemType, itemName, namespace, _, _, _ := OutputItemFromBlock(
		"read_url",
		json.RawMessage(`{"url":"https://example.com/report"}`),
		toolMap,
	)

	if itemType != "function_call" || itemName != "read_url" || namespace != "mcp__catalyst_search__" {
		t.Fatalf("bare alias restored as %q/%q/%q, want function_call/mcp__catalyst_search__/read_url", itemType, namespace, itemName)
	}
}

func TestOutputItemFromBlockRejectsAmbiguousBareFunctionAlias(t *testing.T) {
	toolMap := ToolMap{
		"mcp__catalyst_search__read_url": {Kind: ToolFunction, OpenAIName: "read_url", Namespace: "mcp__catalyst_search__"},
		"mcp__other__read_url":           {Kind: ToolFunction, OpenAIName: "read_url", Namespace: "mcp__other__"},
	}

	_, itemName, namespace, _, _, _ := OutputItemFromBlock("read_url", json.RawMessage(`{}`), toolMap)

	if itemName != "read_url" || namespace != "" {
		t.Fatalf("ambiguous bare alias restored as %q/%q, want unqualified read_url", namespace, itemName)
	}
}

func TestRebuildApplyPatchGrammarUpdateFileIncludesValidPatchMarkers(t *testing.T) {
	input := json.RawMessage(`{
		"path":"internal/example.go",
		"move_to":"internal/example_v2.go",
		"hunks":[
			{
				"context":"func demo()",
				"lines":[
					{"op":"context","text":"func demo() {"},
					{"op":"remove","text":"\told()"},
					{"op":"add","text":"\tnew()"},
					{"op":"context","text":"}"}
				]
			}
		]
	}`)

	got := RebuildApplyPatchGrammar("apply_patch_update_file", input)

	for _, want := range []string{
		"*** Begin Patch\n",
		"*** Update File: internal/example.go\n",
		"*** Move to: internal/example_v2.go\n",
		"@@ func demo()\n",
		" func demo() {\n",
		"-\told()\n",
		"+\tnew()\n",
		" }\n",
		"*** End Patch\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rebuilt patch missing %q:\n%s", want, got)
		}
	}
}

func TestRebuildApplyPatchGrammarBatchPreservesAllOperations(t *testing.T) {
	input := json.RawMessage(`{
		"operations":[
			{"type":"add_file","path":"new.txt","content":"hello\nworld"},
			{"type":"delete_file","path":"old.txt"},
			{
				"type":"update_file",
				"path":"edit.txt",
				"hunks":[
					{
						"context":"header",
						"lines":[
							{"op":"context","text":"same"},
							{"op":"add","text":"added"}
						]
					}
				]
			}
		]
	}`)

	got := RebuildApplyPatchGrammar("apply_patch_batch", input)

	if strings.Count(got, "*** Begin Patch\n") != 3 {
		t.Fatalf("expected 3 begin markers, got:\n%s", got)
	}
	for _, want := range []string{
		"*** Add File: new.txt\n+hello\n+world\n*** End Patch\n",
		"*** Delete File: old.txt\n*** End Patch\n",
		"*** Update File: edit.txt\n@@ header\n same\n+added\n*** End Patch\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("rebuilt batch missing %q:\n%s", want, got)
		}
	}
}

func TestRebuildGrammarUsesRawInputForGenericCustomTools(t *testing.T) {
	got := RebuildGrammar("custom_tool", json.RawMessage(`{"input":"plain freeform body"}`))
	if got != "plain freeform body" {
		t.Fatalf("RebuildGrammar() = %q, want raw input", got)
	}
}
