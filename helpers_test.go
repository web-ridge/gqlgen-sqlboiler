package gbgen

import "testing"

func Test_gopathImport(t *testing.T) {
	type args struct {
		dir string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "in GOPATH",
			args: args{
				dir: "/Users/someonefamous/go/src/github.com/someonefamous/famous-project",
			},
			want: "github.com/someonefamous/famous-project",
		},
	}
	for i := range tests {
		tt := tests[i]
		t.Run(tt.name, func(t *testing.T) {
			if got := gopathImport(tt.args.dir); got != tt.want {
				t.Errorf("gopathImport() = %v, want %v", got, tt.want)
			}
		})
	}
}
