package main

import (
	"bytes"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

var (
	reqId uint64

	rootPage = template.Must(template.New("root").Parse(`
<h1>Orchid Genealogy Plotter</h1>
<form action="/plot" method="POST">
	<div>Grexes (comma-separated: Aladin, Madame Martinet)
	<div><input name="grexes" type="text"></div>
	<div><input type="submit" value="Go"></div>
</form>
`))
)

func dottify(graph string, format string) (output []byte, err error) {
	buf := bytes.NewBuffer([]byte(graph))
	cmd := exec.Command("dot", "/dev/stdin", "-T", format)
	cmd.Stdin = buf
	return cmd.Output()
}

func showError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(msg))

}

func rootHandler(w http.ResponseWriter, req *http.Request) {
	rootPage.Execute(w, "")
}

func plotHandler(reg *Register) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		reqId := atomic.AddUint64(&reqId, 1)

		log.Printf("start_request,%d,%s,%s,%v", reqId, req.RemoteAddr, req.Method, req.URL)
		t0 := time.Now()

		var names []string
		seen := make(map[string]bool)
		formNames := req.FormValue("grexes")
		for _, name := range strings.Split(formNames, ",") {
			name = strings.TrimSpace(name)
			if len(name) > 0 && !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
		switch {
		case len(names) == 0:
			showError(w, "no grex specified")
			return
		case len(names) > 5:
			showError(w, "too many grexes specified")
			return
		}

		format := "jpg"
		g, err := reg.Pull(names)
		if err != nil {
			showError(w, err.Error())
			return
		}

		dot, err := reg.Plot(g, names)
		if err == nil {
			var out []byte
			if out, err = dottify(dot, format); err == nil {
				http.ServeContent(w, req, "image."+format, time.Time{}, bytes.NewReader(out))
			}
		} else {
			showError(w, fmt.Sprintf("something's wrong with the RHS register: %s", err))
		}

		elapsed := time.Since(t0)
		log.Printf("end_request,%d,dt=%v err=%v", reqId, elapsed, err)
	}
}

func main() {
	var (
		port     int
		register string
	)
	flag.StringVar(&register, "rhs", "etc/paphiopedilum.csv", "RHS orchid register")
	flag.IntVar(&port, "port", 0, "http listen port")
	flag.Parse()

	if register == "" {
		flag.Usage()
		os.Exit(0)
	}

	f, err := os.Open(register)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	reg, err := ReadRegister(f)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%d grexes read", reg.Size())

	switch {
	case port > 0:
		log.Printf("listening on port %d", port)
		http.HandleFunc("/", rootHandler)
		http.HandleFunc("/plot", plotHandler(reg))
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			log.Fatal(err)
		}
		log.Println("DONE")
	default:
		names := []string{"Hestia"}
		g, err := reg.Pull(names)
		if err != nil {
			log.Fatal(err)
		}

		dot, err := reg.Plot(g, names)
		if err != nil {
			log.Fatal(err)
		}
		out, err := dottify(dot, "dot")
		fmt.Println(string(out), err)
	}
}
