package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

type Grex struct {
	Name       string
	Seed       string
	Pollen     string
	Originator string
	Date       string
}

type Register struct {
	Grexes map[string]Grex
}

func (r Register) Size() int {
	return len(r.Grexes)
}

func (r Register) FuzzyLookup(name string) (Grex, bool) {
	lower := strings.ToLower(name)
	for _, g := range r.Grexes {
		if lower == strings.ToLower(g.Name) {
			return g, true
		}
	}
	return Grex{}, false
}

func (r Register) Lookup(name string) (Grex, bool) {
	g, ok := r.Grexes[name]
	return g, ok
}

func (r Register) Pull(grexes []string) (graph map[string][]string, err error) {
	graph = make(map[string][]string)

	todo := make(map[string]bool)
	for _, grex := range grexes {
		g, ok := r.FuzzyLookup(grex)
		if !ok {
			return nil, fmt.Errorf("not found: %s", grex)
		}
		todo[g.Name] = true
	}

	unknown := make(map[string]bool)
	for len(todo) > 0 {
		for name := range todo {
			g, ok := r.Lookup(name)
			if !ok {
				return nil, fmt.Errorf("not found: %s", name)
			}
			graph[g.Name] = []string{g.Seed, g.Pollen}

			if seed, ok1 := r.Lookup(g.Seed); ok1 {
				if _, ok := graph[seed.Name]; !ok {
					todo[seed.Name] = true
				}
			} else {
				unknown[g.Seed] = true
			}

			if pollen, ok2 := r.Lookup(g.Pollen); ok2 {
				if _, ok := graph[pollen.Name]; !ok {
					todo[pollen.Name] = true
				}
			} else {
				unknown[g.Pollen] = true
			}
			delete(todo, name)
		}
	}
	for name := range unknown {
		graph[name] = nil
	}

	return
}

func (r Register) Plot(graph map[string][]string, sources []string) (output string, err error) {
	highlight := make(map[string]bool)
	for _, name := range sources {
		highlight[strings.ToLower(name)] = true
	}

	layers, err := Sort(graph)
	if err != nil {
		return "", err
	}

	id := make(map[string]int)

	buff := new(bytes.Buffer)
	fmt.Fprintf(buff, "digraph {\n")
	for _, layer := range layers {
		fmt.Fprintf(buff, "\t{rank=same;\n")
		for _, node := range layer {
			id[node] = len(id)

			var label, style string
			grex, ok := r.Lookup(node)
			switch ok {
			case true:
				label = fmt.Sprintf("%s\\n%s\\n%s", node, grex.Originator, grex.Date)
			case false:
				label = node
			}

			if highlight[strings.ToLower(node)] {
				style = " style=filled"
			}

			fmt.Fprintf(buff, "\t\tN%d [label=\"%s\"%s]\n", id[node], label, style)
		}
		fmt.Fprintf(buff, "\t}\n")
	}

	for node, deps := range graph {
		for _, dep := range deps {
			fmt.Fprintf(buff, "\tN%d -> N%d\n", id[dep], id[node])
		}
	}
	fmt.Fprintf(buff, "}\n")
	return string(buff.Bytes()), nil
}

func Sort(graph map[string][]string) ([][]string, error) {
	todo := make(map[string]bool)

	inc := make(map[string]int)
	for name, deps := range graph {
		inc[name] = len(deps)
		if len(deps) == 0 {
			todo[name] = true
		}
	}
	out := make(map[string][]string)
	for node, deps := range graph {
		for _, x := range deps {
			out[x] = append(out[x], node)
		}
	}

	height := make(map[string]int)
	maxHeight := 0
	for len(todo) > 0 {
		for node := range todo {
			delete(todo, node)
			delete(inc, node)

			h := -1
			for _, dep := range graph[node] {
				if h < height[dep] {
					h = height[dep]
				}
			}
			height[node] = h + 1
			if height[node] > maxHeight {
				maxHeight = height[node]
			}

			for _, x := range out[node] {
				inc[x] = inc[x] - 1
				if inc[x] == 0 {
					todo[x] = true
					delete(inc, node)
				}
			}
		}
	}

	layers := make([][]string, maxHeight+1)
	for node, h := range height {
		layers[h] = append(layers[h], node)
	}

	if len(inc) > 0 {
		return nil, fmt.Errorf("graph has cycles")
	}
	return layers, nil
}

func ReadRegister(rd io.Reader) (*Register, error) {
	cr := csv.NewReader(rd)
	cr.Comma = ';'

	idx := make(map[string]int)
	header, err := cr.Read()
	if err != nil {
		return nil, err
	}
	for i, col := range header {
		idx[strings.ToUpper(col)] = i
	}

	grexes := make(map[string]Grex)
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, err
		}

		t := strings.Split(row[idx["DATE OF REGISTRATION"]], "/")
		grex := Grex{
			Name:       row[idx["EPITHET"]],
			Seed:       row[idx["SEED"]],
			Pollen:     row[idx["POLLEN"]],
			Originator: row[idx["ORIGINATOR NAME"]],
			Date:       t[len(t)-1],
		}
		grexes[grex.Name] = grex
	}
	return &Register{grexes}, nil
}
