package chart

import (
	"errors"
	"regexp"
	"sort"
	"strings"

	ketchv1 "github.com/shipa-corp/ketch/internal/api/v1beta1"
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
	return &Procfile{
		Processes:           processes,
		RoutableProcessName: routableProcess(names),
	}, nil
}

// ProcfileFromProcesses construct a Procfile instance from a list of ProcessSpec and returns it.
func ProcfileFromProcesses(processes []ketchv1.ProcessSpec) (*Procfile, error) {
	if len(processes) == 0 {
		return nil, ErrEmptyProcfile
	}
	procfile := Procfile{
		Processes: make(map[string][]string, len(processes)),
	}
	var names []string
	for _, spec := range processes {
		procfile.Processes[spec.Name] = spec.Cmd
		names = append(names, spec.Name)
	}
	procfile.RoutableProcessName = routableProcess(names)
	return &procfile, nil
}

func routableProcess(names []string) string {
	for _, name := range names {
		if name == DefaultRoutableProcessName {
			return DefaultRoutableProcessName
		}
	}
	sort.Strings(names)
	return names[0]
}
