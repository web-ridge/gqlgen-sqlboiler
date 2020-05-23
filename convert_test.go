// testing really need more improvements!
// Please add more tests if you use this

package gqlgen_sqlboiler

import "testing"

func TestShortType(t *testing.T) {
	testShortType(t, "gitlab.com/product/app/backend/graphql_models.FlowWhere", "FlowWhere")
	testShortType(t, "*gitlab.com/product/app/backend/graphql_models.FlowWhere", "*FlowWhere")
	testShortType(t, "*github.com/web-ridge/go-utils/boilergql/boilergql.GeoPoint", "*GeoPoint")
	testShortType(t, "github.com/web-ridge/go-utils/boilergql/boilergql.GeoPoint", "GeoPoint")
	testShortType(t, "*string", "*string")
	testShortType(t, "string", "string")
	testShortType(t, "*time.Time", "*time.Time")
}

func testShortType(t *testing.T, input, output string) {
	result := getShortType(input)
	if result != output {
		t.Errorf("%v should result in %v but did result in %v", input, output, result)
	}
}

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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gopathImport(tt.args.dir); got != tt.want {
				t.Errorf("gopathImport() = %v, want %v", got, tt.want)
			}
		})
	}
}
