package main

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
)

const BLOCK_SIZE = 4086

const fileHeader = `package {{.Package}}

import (
	"bytes"
	"compress/gzip"
	"io"
)
`
const blockHeader = `
// {{.Filename}}
const z_{{.Variable}} = []byte{
`

const blockFooter = `}

func {{.Variable}}() ([]byte, error) {
	var in = bytes.NewReader(z_{{.Variable}})
	var out bytes.Buffer
	z, err := gzip.NewReader(&in)
	if err != nil {
		return nil, err
	}
	io.Copy(&out, z)
	return out.Bytes(), nil
}
`

type Config struct {
	Package string
	Output  string

	noGoGenerate bool
	append       bool
}

type Header struct {
	Package string
}

type Data struct {
	Variable, Filename string
}

func main() {
	var fout *os.File
	var err error

	c := &Config{}

	flag.StringVar(&c.Package, "p", "", "package name")
	flag.StringVar(&c.Output, "o", "", "output file")

	flag.BoolVar(&c.noGoGenerate, "G", false, "omit //go:generate")
	flag.BoolVar(&c.append, "a", false, "append to existing file")

	flag.Parse()

	// per-output
	h := Header{
		Package: c.Package,
	}
	th0 := template.Must(template.New("header").Parse(fileHeader))

	// per-input
	th1 := template.Must(template.New("block_header").Parse(blockHeader))
	tf1 := template.Must(template.New("block_footer").Parse(blockFooter))

	if len(c.Output) > 0 {
		// single output
		flags := os.O_CREATE | os.O_WRONLY
		if c.append {
			flags |= os.O_APPEND
		}

		fout, err = os.OpenFile(c.Output, flags, 0644)
		if err != nil {
			panic(err)
		}

		if !c.noGoGenerate {
			var s []string
			s = append(s, fmt.Sprintf("//go:generate %s -p %s -o %s", os.Args[3], h.Package, c.Output))
			if c.append {
				s = append(s, "-a")
			}
			for _, fname := range flag.Args() {
				s = append(s, fname)
			}
			fout.WriteString(strings.Join(s, " "))
			fout.WriteString("\n\n")
		}

		if err = th0.Execute(fout, h); err != nil {
			panic(err)
		}
	}

	for _, fname := range flag.Args() {
		var buf bytes.Buffer
		var varname = fname

		varname = strings.Replace(varname, ".", "_", -1)
		varname = strings.Replace(varname, "/", "_", -1)
		varname = strings.Replace(varname, "-", "_", -1)

		// input file
		f, err := os.Open(fname)
		if err != nil {
			panic(err)
		}

		// compress
		z, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
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
		if len(c.Output) == 0 {
			outname := fname + ".go"
			flags := os.O_CREATE | os.O_WRONLY
			if c.append {
				flags |= os.O_APPEND
			}

			if fout != nil {
				fout.Close()
			}

			fout, err = os.OpenFile(outname, flags, 0644)
			if err != nil {
				panic(err)
			}

			if !c.noGoGenerate {
				var s []string
				s = append(s, fmt.Sprintf("//go:generate %s -p %s", os.Args[0], h.Package))
				if c.append {
					s = append(s, "-a")
				}
				s = append(s, fname)
				fout.WriteString(strings.Join(s, " "))
				fout.WriteString("\n\n")
			}

			if err = th0.Execute(fout, h); err != nil {
				panic(err)
			}
		}

		d := Data{
			Filename: fname,
			Variable: varname,
		}

		if err = th1.Execute(fout, d); err != nil {
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
			fmt.Fprintf(fout, "%s0x%02x%s", pre, b, post)
		}

		if err = tf1.Execute(fout, d); err != nil {
			panic(err)
		}
	}

	if fout != nil {
		fout.Close()
	}
}
