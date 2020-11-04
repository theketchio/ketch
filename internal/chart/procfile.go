package chart

import (
	"errors"
	"regexp"
	"sort"
	"strings"
)

var (
	ErrEmptyProcfile = errors.New("procfile should contain at least one process name with a command")

	procfileRegex = regexp.MustCompile(`^([A-Za-z0-9_-]+):\s*(.+)$`)
)

// Procfile represents a parsed Procfile.
type Procfile struct {
	Processes           map[string][]string
	RoutableProcessName string
}

func (p *Procfile) IsRoutable(processName string) bool {
	return p.RoutableProcessName == processName
}

func (p *Procfile) SortedNames() []string {
	names := make([]string, 0, len(p.Processes))
	for name := range p.Processes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ParseProcfile parses the content of Procfile and returns a Procfile instance.
func ParseProcfile(content string) (*Procfile, error) {
	procfile := strings.Split(content, "\n")
	processes := make(map[string][]string, len(procfile))
	var names []string
	for _, process := range procfile {
		if p := procfileRegex.FindStringSubmatch(process); p != nil {
			name := p[1]
			cmd := p[2]
			processes[name] = []string{strings.TrimSpace(cmd)}
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, ErrEmptyProcfile
	}
	sort.Strings(names)
	routableProcessName := names[0]

	if _, ok := processes[DefaultRoutableProcessName]; ok {
		routableProcessName = DefaultRoutableProcessName
	}
	return &Procfile{
		Processes:           processes,
		RoutableProcessName: routableProcessName,
	}, nil
}
