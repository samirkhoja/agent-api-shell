package apishell

import "sort"

type Args struct {
	values map[string][]string
}

func NewArgs(values map[string][]string) Args {
	return Args{values: cloneArgMap(values)}
}

func (a Args) Value(name string) (string, bool) {
	values := a.values[name]
	if len(values) == 0 {
		return "", false
	}
	return values[0], true
}

func (a Args) Values(name string) []string {
	return cloneStringSlice(a.values[name])
}

func (a Args) Has(name string) bool {
	return len(a.values[name]) > 0
}

func (a Args) Names() []string {
	names := make([]string, 0, len(a.values))
	for name := range a.values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (a Args) Map() map[string][]string {
	return cloneArgMap(a.values)
}

func cloneArgMap(input map[string][]string) map[string][]string {
	if len(input) == 0 {
		return map[string][]string{}
	}
	cloned := make(map[string][]string, len(input))
	for key, values := range input {
		cloned[key] = cloneStringSlice(values)
	}
	return cloned
}

func cloneStringSlice(input []string) []string {
	if len(input) == 0 {
		return nil
	}
	cloned := make([]string, len(input))
	copy(cloned, input)
	return cloned
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func cloneResultMetadata(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}
