package main

import (
	"bytes"
	"compress/zlib"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const BLOCK_SIZE = 4086

func main() {
	pkgname := flag.String("p", "", "package name")

	flag.Parse()

	for _, fname := range flag.Args() {
		var buf bytes.Buffer
		var varname = strings.Replace(fname, ".", "_", -1)

		// input file
		f, err := os.Open(fname)
		if err != nil {
			panic(err)
		}

		// compress
		z, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
		if err != nil {
			panic(err)
		}

		data := make([]byte, BLOCK_SIZE)
		for {
			data = data[:cap(data)]
			n, err := f.Read(data)
			if err != nil {
				if err == io.EOF {
					break
				}
				panic(err)
			}
			z.Write(data[:n])
		}
		z.Flush()
		z.Close()

		f.Close()

		// output file
		f, err = os.Create(fname + ".go")
		if err != nil {
			panic(err)
		}

		fmt.Fprintf(f, "package %s\n\n//go:generate file2govar -p %s %s\n\n", *pkgname, *pkgname, fname)
		fmt.Fprintf(f, "import (\n\t\"bytes\"\n\t\"io\"\n)\n\n")
		fmt.Fprintf(f, "const z_%s = []byte{\n", varname)
		for i, b := range buf.Bytes() {
			var pre, post string
			var col = i % 8

			if col == 7 {
				post = ",\n"
			}

			if col == 0 {
				pre = "\t"
			} else {
				pre = ", "
			}
			fmt.Fprintf(f, "%s0x%02x%s", pre, b, post)
		}
		fmt.Fprintf(f, "}\n\n")
		fmt.Fprintf(f, "func %s() ([]byte, error) {\n", varname)
		fmt.Fprintf(f, "\tvar in = bytes.NewReader(z_%s)\n", varname)
		fmt.Fprintf(f, "\tvar out bytes.Buffer\n")
		fmt.Fprintf(f, "\tz, err := zlib.NewReader(&in)\n")
		fmt.Fprintf(f, "\tif err != nil {\n\t\treturn nil, err\n\t}\n")
		fmt.Fprintf(f, "\tio.Copy(&out, z)\n")
		fmt.Fprintf(f, "\treturn out.Bytes(), nil\n}\n")
		f.Close()
	}
}
