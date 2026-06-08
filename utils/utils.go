// spec: spec/modules.md

package utils

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func StreamPipe(std io.ReadCloser) {
	defer std.Close()
	buf := bufio.NewReader(std)
	for {
		line, _, err := buf.ReadLine()
		if err != nil {
			break
		}
		fmt.Println(string(line))
	}
}

func StreamCommand(cmd *exec.Cmd) error {
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
		StreamPipe(stdout)
	}()

	go func() {
		defer wg.Done()
		StreamPipe(stderr)
	}()

	wg.Wait()
	return cmd.Wait()
}

func DedupeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}


