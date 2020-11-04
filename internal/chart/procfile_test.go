package chart

import (
	"reflect"
	"testing"
)

func TestParseProcfile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    *Procfile
		wantErr bool
	}{
		{
			name:    "single web command",
			content: "web: command arg1 arg2",
			want: &Procfile{
				Processes: map[string][]string{
					"web": {"command arg1 arg2"},
				},
				RoutableProcessName: "web",
			},
		},
		{
			name:    "single command",
			content: "long-command-name: command arg1 arg2",
			want: &Procfile{
				Processes: map[string][]string{
					"long-command-name": {"command arg1 arg2"},
				},
				RoutableProcessName: "long-command-name",
			},
		},
		{
			name:    "two commands",
			content: "web: command arg1 arg2\nworker: celery worker",
			want: &Procfile{
				Processes: map[string][]string{
					"web":    {"command arg1 arg2"},
					"worker": {"celery worker"},
				},
				RoutableProcessName: "web",
			},
		},
		{
			name:    "two commands without web",
			content: "worker: command arg1 arg2\n\r\nabc: abc-arg1 abc-arg2",
			want: &Procfile{
				Processes: map[string][]string{
					"worker": {"command arg1 arg2"},
					"abc":    {"abc-arg1 abc-arg2"},
				},
				RoutableProcessName: "abc",
			},
		},
		{
			name:    "three commands without web",
			content: "bbb: bbb-command\n\r\nzzz: zzz-command\r\naaa: aaa-command",
			want: &Procfile{
				Processes: map[string][]string{
					"aaa": {"aaa-command"},
					"zzz": {"zzz-command"},
					"bbb": {"bbb-command"},
				},
				RoutableProcessName: "aaa",
			},
		},
		{
			name:    "procfile with comments",
			content: "bbb: bbb-command\n# some comment\n\nzzz: zzz-command\r\naaa: aaa-command\n # another comment",
			want: &Procfile{
				Processes: map[string][]string{
					"aaa": {"aaa-command"},
					"zzz": {"zzz-command"},
					"bbb": {"bbb-command"},
				},
				RoutableProcessName: "aaa",
			},
		},
		{
			name:    "ingore broken lines",
			content: "b,bb: bbb-command\n\r\n: zzz-command\r\naaa: aaa-command",
			want: &Procfile{
				Processes: map[string][]string{
					"aaa": {"aaa-command"},
				},
				RoutableProcessName: "aaa",
			},
		},
		{
			name:    "broken procfile",
			content: ": bbb-command",
			wantErr: true,
		},
		{
			name:    "empty procfile",
			content: "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseProcfile(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseProcfile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseProcfile() got = %v, want %v", got, tt.want)
			}
		})
	}
}
