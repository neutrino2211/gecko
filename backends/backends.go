// spec: spec/types.md, spec/functions.md, spec/classes.md, spec/traits.md, spec/generics.md, spec/control-flow.md, spec/operators.md, spec/pointers.md, spec/memory.md, spec/c-interop.md, spec/attributes.md

package backends

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/neutrino2211/gecko/ast"
	"github.com/neutrino2211/gecko/interfaces"
	"github.com/neutrino2211/gecko/tokens"
)

func streamPipe(std io.ReadCloser) {
	defer std.Close()
	buf := bufio.NewReader(std) // Notice that this is not in a loop
	for {

		line, _, err := buf.ReadLine()
		if err != nil {
			break
		}
		fmt.Println(string(line))
	}
}

func streamCommand(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	err = cmd.Start()
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		streamPipe(stdout)
	}()

	go func() {
		defer wg.Done()
		streamPipe(stderr)
	}()

	wg.Wait()
	return cmd.Wait()
}

func BackendRun(b interfaces.BackendInteface, c *interfaces.BackendConfig) *exec.Cmd {
	return b.Compile(c)
}

func BackendProcessEntries(b interfaces.BackendInteface, scope *ast.Ast, entries []*tokens.Entry) {
	impls := b.GetImpls()
	if impls == nil {
		return
	}

	pipeline := newSharedLoweringPipeline(newCompatibilityEmitter(impls))
	pipeline.EmitEntries(scope, entries)
}

var Backends = map[string]interfaces.BackendInteface{
	"llvm": &LLVMBackend{},
	"c":    &CBackend{},
	"asm":  &AsmBackend{},
}
