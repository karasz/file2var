package main

import (
	"bytes"
	"compress/zlib"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
)

const BLOCK_SIZE = 4086

const header = `//go:generate file2govar -p {{.Package}} {{.Filename}}

package {{.Package}}

import (
	"bytes"
	"compress/zlib"
	"io"
)

const z_{{.Variable}} = []byte{
`

const footer = `}

func {{.Variable}}() ([]byte, error) {
	var in = bytes.NewReader(z_{{.Variable}})
	var out bytes.Buffer
	z, err := zlib.NewReader(&in)
	if err != nil {
		return nil, err
	}
	io.Copy(&out, z)
	return out.Bytes(), nil
}
`

type Data struct {
	Package, Filename, Variable string
}

func main() {
	pkgname := flag.String("p", "", "package name")

	flag.Parse()

	th := template.Must(template.New("header").Parse(header))
	tf := template.Must(template.New("footer").Parse(footer))

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
		f.Close()
		z.Flush()
		z.Close()

		// output file
		f, err = os.Create(fname + ".go")
		if err != nil {
			panic(err)
		}

		d := Data{
			Package:  *pkgname,
			Filename: fname,
			Variable: varname,
		}

		if err = th.Execute(f, d); err != nil {
			panic(err)
		}

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

		if err = tf.Execute(f, d); err != nil {
			panic(err)
		}

		f.Close()
	}
}
