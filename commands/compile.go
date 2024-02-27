package commands

import (
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/neutrino2211/gecko/compiler"
	"github.com/neutrino2211/gecko/config"
	"github.com/urfave/cli/v2"

	"github.com/fatih/color"
)

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherise, attempt to create a hard link
// between the two files. If that fail, copy the file contents from src to dst.
func CopyFile(src, dst string) (err error) {
	sfi, err := os.Stat(src)
	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}
	dfi, err := os.Stat(dst)
	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
	} else {
		if !(dfi.Mode().IsRegular()) {
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if err = os.Link(src, dst); err == nil {
		return
	}
	err = copyFileContents(src, dst)
	return
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

var CompileCommand = &cli.Command{
	Name:        "compile",
	Aliases:     []string{"c"},
	Usage:       "gecko compile ...sources",
	Description: compileHelp,
	Action: func(ctx *cli.Context) error {
		if ctx.Args().Len() == 0 {
			println("No sources provided")
			return nil
		}

		for _, pos := range ctx.Args().Slice() {
			outFile := compiler.Compile(pos, &config.CompileCfg{
				Arch:     ctx.String("target-arch"),
				Platform: ctx.String("target-platform"),
				Vendor:   ctx.String("target-vendor"),
				CFlags:   []string{},
				CLFlags:  []string{},
				CObjects: []string{},
				Ctx:      ctx,
			})

			if outFile != "" {
				CopyFile(outFile, pos+".o")
			}
		}

		compiler.PrintErrorSummary()

		return nil
	},
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "output-dir",
			Value: ".",
			Usage: "Output directory path " + color.HiYellowString("(warning: this overrides the build configuration's output directory)"),
		},
		&cli.StringFlag{
			Name:  "type",
			Value: "executable",
			Usage: "Output type for program. (executable | library)",
		},
		&cli.StringFlag{
			Name:  "backend",
			Value: "llvm",
			Usage: "The compilation backend to use (llvm | c)",
		},
		&cli.StringFlag{
			Name:  "target-arch",
			Value: runtime.GOARCH,
			Usage: "The compilation target architecture",
		},
		&cli.StringFlag{
			Name:  "target-platform",
			Value: runtime.GOOS,
			Usage: "The compilation target operating system",
		},
		&cli.StringFlag{
			Name:  "target-vendor",
			Value: "",
			Usage: "The compilation target vendor of file type",
		},
		&cli.BoolFlag{
			Name:  "print-ir",
			Value: false,
			Usage: "Print the file's LLVM IR",
		},
		&cli.BoolFlag{
			Name:  "ir-only",
			Value: false,
			Usage: "Only compile to IR",
		},
		&cli.StringSliceFlag{
			Name:  "llc-args",
			Value: &cli.StringSlice{},
			Usage: "Pass arguments to underlying llc command",
		},
		&cli.BoolFlag{
			Name:  "print-c",
			Value: false,
			Usage: "Print the file's C source",
		},
		&cli.BoolFlag{
			Name:  "c-only",
			Value: false,
			Usage: "Only compile to C",
		},
		&cli.StringSliceFlag{
			Name:  "clang-args",
			Value: &cli.StringSlice{},
			Usage: "Pass arguments to underlying clang command",
		},
	},
}

var (
	compileHelp  = `compiles a gecko source file or a gecko project`
	invokeDir, _ = os.Getwd()
)
