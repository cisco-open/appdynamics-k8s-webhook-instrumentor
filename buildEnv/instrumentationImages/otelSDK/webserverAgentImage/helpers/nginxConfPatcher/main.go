package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aluttik/go-crossplane"
)

type PatchOps []PatchOp

type PatchOp struct {
	Op    string               `json:"op"`
	Path  string               `json:"path"`
	Value crossplane.Directive `json:"value"`
}

type DirectiveExt struct {
	Directive string          `json:"directive"`
	Line      int             `json:"line"`
	Args      []string        `json:"args"`
	Includes  *[]int          `json:"includes,omitempty"`
	Block     *[]DirectiveExt `json:"block,omitempty"`
	Comment   *string         `json:"comment,omitempty"`
	Idx       int             `json:"idx"`
}

func fromDirective(d *crossplane.Directive) *DirectiveExt {

	var blockExt *[]DirectiveExt
	idx := 0
	if d.IsBlock() {
		blockExt = &[]DirectiveExt{}
		dirCounts := map[string]int{}
		for _, blkDir := range *d.Block {
			if _, ok := dirCounts[blkDir.Directive]; !ok {
				dirCounts[blkDir.Directive] = 0
			} else {
				dirCounts[blkDir.Directive]++
			}
			blkDirExt := *fromDirective(&blkDir)
			blkDirExt.Idx = dirCounts[blkDir.Directive]
			blockSlice := append(*blockExt, blkDirExt)
			blockExt = &blockSlice
		}
	}
	out := DirectiveExt{
		Directive: d.Directive,
		Line:      d.Line,
		Args:      d.Args,
		Includes:  d.Includes,
		Block:     blockExt,
		Comment:   d.Comment,
		Idx:       idx,
	}
	return &out
}

func (d *DirectiveExt) toDirective() *crossplane.Directive {
	var blockExt *[]crossplane.Directive
	if d.Block != nil { // IsBlock
		blockExt = &[]crossplane.Directive{}
		for _, blkDir := range *d.Block {
			blockSlice := append(*blockExt, *blkDir.toDirective())
			blockExt = &blockSlice
		}
	}
	out := crossplane.Directive{
		Directive: d.Directive,
		Line:      d.Line,
		Args:      d.Args,
		Includes:  d.Includes,
		Block:     blockExt,
		Comment:   d.Comment,
	}
	return &out
}

func (d *DirectiveExt) String() string {
	s := ""
	if d.Block != nil {
		s = s + fmt.Sprintf("{ Directive: %s, Args: %v, Idx: %d }\n", d.Directive, d.Args, d.Idx)
		for _, b := range *d.Block {
			s = s + b.String()
		}
	} else {
		s = fmt.Sprintf("{ Directive: %s, Args: %v, Idx: %d }\n", d.Directive, d.Args, d.Idx)
	}
	return s
}

func (d *DirectiveExt) Paths(path string) string {
	s := ""
	if d.Block != nil {
		currPath := path + "/" + fmt.Sprintf("%s/%d", d.Directive, d.Idx)
		s = s + fmt.Sprintf("{ Path: %s, Directive: %s, Args: %v, Idx: %d }\n", currPath, d.Directive, d.Args, d.Idx)
		for _, b := range *d.Block {
			s = s + b.Paths(currPath)
		}
	} else {
		currPath := path + "/" + fmt.Sprintf("%s/%d", d.Directive, d.Idx)
		s = fmt.Sprintf("{ Path: %s, Directive: %s, Args: %v, Idx: %d }\n", currPath, d.Directive, d.Args, d.Idx)
	}
	return s
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Required 3 args - config-file, path-to-otel-library, path-to-otel-config\n")
		os.Exit(2)
	}
	path := os.Args[1]
	loadModule := os.Args[2]
	otelConfig := os.Args[3]

	var nginxConf *crossplane.Payload
	nginxConf, err := crossplane.Parse(path, &crossplane.ParseOptions{ParseComments: true, SingleFile: true})
	if err != nil {
		panic(err)
	}

	/*
		nginxJsonBytes, err := json.MarshalIndent(nginxConf, "", "  ")
		if err != nil {
			panic(err)
		}

		fmt.Println(string(nginxJsonBytes))
	*/

	rootNginxConfig := crossplane.Directive{
		Directive: "@root@",
		Block:     &nginxConf.Config[0].Parsed,
		Line:      0,
		Args:      []string{},
	}

	patchJsonStr := `[
		{ "op": "add", 
			"path": "/@root@/0/-", 
			"value": {
				"directive": "load_module",
				"args": ["%s"]
			}
		},
		{ "op": "add", 
			"path": "/@root@/0/http/0/-", 
			"value": {
				"directive": "include",
				"args": ["%s"]
			}
		}
	]`

	patchJsonBytes := []byte(fmt.Sprintf(patchJsonStr, loadModule, otelConfig))

	var patch PatchOps
	err = json.Unmarshal(patchJsonBytes, &patch)
	if err != nil {
		panic(err)
	}

	expandedNginxRoot := fromDirective(&rootNginxConfig)
	// fmt.Println(expandedNginxRoot.Paths(""))

	patchedExpandedNginxRoot, err := applyPatchOps(expandedNginxRoot, &patch)
	if err != nil {
		panic(err)
	}

	modifiedNginxRoot := patchedExpandedNginxRoot.toDirective()
	// deencapsule from root element
	nginxConf.Config[0].Parsed = *modifiedNginxRoot.Block

	var buf bytes.Buffer
	if err = crossplane.Build(&buf, nginxConf.Config[0], &crossplane.BuildOptions{}); err != nil {
		panic(err)
	}

	// print config file to stdout
	fmt.Println(buf.String())
}

func applyPatchOps(config *DirectiveExt, patches *PatchOps) (*DirectiveExt, error) {
	newConfig := &DirectiveExt{}
	var err error

	// clone original config
	jsonClone, _ := json.Marshal(config)
	json.Unmarshal(jsonClone, &newConfig)

	for _, patch := range *patches {
		currentPath := ""
		lineOffset := 0
		newConfig = applyPatch(newConfig, &patch, currentPath, lineOffset)
		if err != nil {
			return nil, err
		}
	}

	return newConfig, nil
}

func applyPatch(dirIn *DirectiveExt, patch *PatchOp, upstreamPath string, lineOffset int) *DirectiveExt {
	lineIncr := 0
	dirOut := DirectiveExt{
		Directive: dirIn.Directive,
		Line:      dirIn.Line,
		Comment:   dirIn.Comment,
		Args:      dirIn.Args,
		Includes:  dirIn.Includes,
	}

	if dirIn.Block != nil {
		currentPath := fmt.Sprintf("%s/%s/%d", upstreamPath, dirIn.Directive, dirIn.Idx)
		if match(currentPath, patch.Path) {
			lineIncr := 1
			toBeInserted := &DirectiveExt{
				Directive: patch.Value.Directive,
				Args:      patch.Value.Args,
			}
			switch placement(patch.Path) {
			case "-":
				toBeInserted.Line = dirIn.Line + lineIncr + lineOffset
				temp := append([]DirectiveExt{*toBeInserted}, *dirIn.Block...)
				dirIn.Block = &temp
			case "+":
				toBeInserted.Line = (*dirIn.Block)[len(*dirIn.Block)-1].Line + lineIncr + lineOffset
				temp := append(*dirIn.Block, *toBeInserted)
				dirIn.Block = &temp
			}
		}
		dirOut.Block = &[]DirectiveExt{}
		for _, directive := range *dirIn.Block {
			patched := applyPatch(&directive, patch, currentPath, lineOffset+lineIncr)
			temp := append(*dirOut.Block, *patched)
			dirOut.Block = &temp
		}
	}
	// fmt.Printf("%s: %s\n", dirIn.Directive, dirIn.String())
	return &dirOut
}

func match(currPath, seekedPath string) bool {
	seekedPathSegs := strings.Split(seekedPath, "/")
	result := currPath == strings.Join(seekedPathSegs[:len(seekedPathSegs)-1], "/")
	// fmt.Printf("Matching '%s' <-> '%s' => %t\n", currPath, seekedPath, result)
	return result
}

func placement(seekedPath string) string {
	segs := strings.Split(seekedPath, "/")
	return segs[len(segs)-1]
}
